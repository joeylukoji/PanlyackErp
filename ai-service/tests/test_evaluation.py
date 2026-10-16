import asyncio
import json

import pytest
from PIL import Image

from app.evaluation import prepare, run, summarize
from app.schemas import Invoice


def test_prepare_selects_unique_images_reproducibly_and_preserves_references(tmp_path):
    source = tmp_path / 'source'
    source.mkdir()
    for index in range(6):
        Image.new('RGB', (20 + index, 30), (index * 30, 0, 0)).save(source / f'{index}.png')
    (source / 'duplicate.png').write_bytes((source / '0.png').read_bytes())
    first = prepare(source, tmp_path / 'first', count=6)
    second = prepare(source, tmp_path / 'second', count=6)
    assert [row['source_sha256'] for row in first] == [row['source_sha256'] for row in second]
    assert len({row['source_sha256'] for row in first}) == 6
    assert all(row['reference'] is None and row['review_status'] == 'pending' for row in first)
    with pytest.raises(FileExistsError):
        prepare(source, tmp_path / 'first', count=6)
    with pytest.raises(ValueError, match='distinct'):
        prepare(source, tmp_path / 'too-large', count=7)


def test_scores_only_human_confirmed_fields_and_exact_decimal_amounts():
    rows = [
        {'id': '1', 'review_status': 'confirmed', 'reviewed_by': 'Reviewer',
         'reference': {'total': '900719925474.123456', 'partner': ' Supplier ', 'number': None}},
        {'id': '2', 'review_status': 'pending', 'reference': {'total': '2'}},
        {'id': '3', 'review_status': 'confirmed', 'reviewed_by': '', 'reference': {'total': '3'}},
    ]
    predictions = [
        {'id': '1', 'status': 'ok', 'prediction': {'total': '900719925474.123455', 'partner': 'supplier'}},
        {'id': '2', 'status': 'ok', 'prediction': {'total': '2'}},
        {'id': '3', 'status': 'error'},
    ]
    report = summarize(rows, predictions)
    assert report['confirmed_documents'] == 1
    assert report['scored_fields'] == 2 and report['matched_fields'] == 1
    assert report['fields']['total'] == {'matched': 0, 'scored': 1}
    assert report['pending_documents'] == 2


def test_unreviewed_and_failed_predictions_do_not_produce_fake_accuracy():
    rows = [{'id': '1', 'review_status': 'pending', 'reference': None}]
    report = summarize(rows, [{'id': '1', 'status': 'error'}])
    assert report['field_accuracy'] is None
    assert report['successful_extractions'] == 0
    assert report['failed_extractions'] == 1
    rows[0].update(review_status='confirmed', reviewed_by='Reviewer', reference={'total': '12.00'})
    report = summarize(rows, [{'id': '1', 'status': 'error'}])
    assert report['matched_fields'] == 0 and report['scored_fields'] == 1


def test_line_count_and_decimal_line_fields_are_scored():
    rows = [{'id': '1', 'review_status': 'confirmed', 'reviewed_by': 'Reviewer',
             'reference': {'lines': [{'quantity': '2.0', 'unit_price': '3.10'}]}}]
    predictions = [{'id': '1', 'status': 'ok', 'prediction': {
        'lines': [{'quantity': '2', 'unit_price': '3.1'}, {'quantity': '1'}]}}]
    report = summarize(rows, predictions)
    assert report['scored_fields'] == 3 and report['matched_fields'] == 2


def test_failed_extraction_cannot_match_an_empty_reference_line_list():
    rows = [{'id': '1', 'review_status': 'confirmed', 'reviewed_by': 'Reviewer',
             'reference': {'lines': []}}]
    for predictions in ([], [{'id': '1', 'status': 'error'}]):
        report = summarize(rows, predictions)
        assert report['scored_fields'] == 1 and report['matched_fields'] == 0


def test_runner_records_failures_continues_and_never_changes_annotations(tmp_path):
    source = tmp_path / 'source'
    source.mkdir()
    for index in range(2):
        Image.new('RGB', (20 + index, 30)).save(source / f'{index}.png')
    dataset = tmp_path / 'dataset'
    prepare(source, dataset, count=2)
    before = (dataset / 'annotations.json').read_bytes()

    class Extractor:
        calls = 0

        async def extract(self, request):
            self.calls += 1
            if self.calls == 1:
                raise ValueError('Unreadable document')
            return Invoice(number='A', partner='Supplier', issue_date='2026-01-01', total='1',
                           lines=[{'name': 'Item', 'quantity': '1', 'unit_price': '1'}])

    report = asyncio.run(run(dataset, dataset / 'runs' / 'test', Extractor()))
    assert report['attempted_documents'] == 2
    assert report['failed_extractions'] == 1 and report['successful_extractions'] == 1
    assert report['field_accuracy'] is None
    assert (dataset / 'annotations.json').read_bytes() == before
    assert len(json.loads((dataset / 'runs/test/predictions.json').read_text())) == 2
