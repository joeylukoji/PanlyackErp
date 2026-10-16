import csv
import hashlib
import json

import pytest
from PIL import Image

from app.training_review import finalize, prepare, write_guides


def sources(tmp_path):
    local = tmp_path / 'local'
    public = tmp_path / 'public'
    (local / 'outputs/photos_factures/images').mkdir(parents=True)
    (public / 'images').mkdir(parents=True)
    rows = []
    for index in range(4):
        Image.new('RGB', (20, 30), (index * 50, 20, 0)).save(local / f'outputs/photos_factures/images/{index}.jpg')
        Image.new('RGB', (30, 20), (0, index * 50, 20)).save(public / f'images/train-00000-of-00005_{index:05}.tif')
        rows.append({'image': f'images/{index}.jpg', 'type': str(index % 2), 'invoice_no': f'F-{index}',
                     'date': '16/10/2026', 'store': 'Supplier <script>bad()</script>', 'currency': 'USD',
                     'items': [{'name': 'Item', 'qty': 3, 'unit_price': 0.1, 'line_total': 0.3}],
                     'subtotal': 0.3, 'discount': 0, 'tax_rate': 0.1, 'tax': 0.03, 'total': 0.33,
                     'paid': 1, 'change': 0.67})
    root = local / 'outputs/photos_factures'
    (root / 'labels_photos.json').write_text(json.dumps(rows), encoding='utf-8')
    with (root / 'labels_photos.csv').open('w', newline='', encoding='utf-8') as stream:
        writer = csv.DictWriter(stream, fieldnames=['image', 'total'])
        writer.writeheader()
        writer.writerows({'image': row['image'], 'total': row['total']} for row in rows)
    (public / 'progress.json').write_text(json.dumps({'shards': [{'file': 'train-00000-of-00005.parquet', 'rows': 4, 'label_counts': {'6': 4}}]}))
    (public / 'dataset_infos.json').write_text(json.dumps({'config': {'features': {'label': {'names': ['other'] * 6 + ['invoice']}}}}))
    return local, public


def test_prepares_separate_pending_lots_preserving_raw_labels_and_exact_values(tmp_path):
    local, public = sources(tmp_path)
    output = tmp_path / 'review'
    prepare(local, public, output, count=2)
    local_rows = json.loads((output / 'local_50/annotations.json').read_text())
    public_rows = json.loads((output / 'public_50/annotations.json').read_text())
    assert len(local_rows) == len(public_rows) == 2
    for row in local_rows + public_rows:
        assert row['review_status'] == 'pending' and row['reference'] is None
        assert row['reviewed_by'] == '' and row['document_type'] == 'unknown'
    assert local_rows[0]['source_reference_draft']['total'] == '0.33'
    assert local_rows[0]['source_reference_draft']['lines'][0]['unit_price'] == '0.1'
    assert local_rows[0]['source_reference_draft']['tax_rate_percent'] == '10.0'
    assert public_rows[0]['source_class_label'] == 'invoice' and public_rows[0]['source_reference_draft'] is None
    assert (output / 'local_50/source/labels_photos.json').read_bytes() == (local / 'outputs/photos_factures/labels_photos.json').read_bytes()
    assert '&lt;script&gt;' in (output / 'local_50/index.html').read_text(encoding='utf-8')
    assert '<script>bad()' not in (output / 'local_50/index.html').read_text(encoding='utf-8')
    assert len(list((output / 'local_50/images').glob('*'))) == 2
    write_guides(output, public)
    finalize(output)
    assert json.loads((output / 'verification.json').read_text())['distinct_original_hashes'] == 4
    assert (output / 'SHA256SUMS.txt').is_file()
    with pytest.raises(FileExistsError):
        prepare(local, public, output, count=2)


def test_excludes_public_evaluation_hashes_and_refuses_insufficient_images(tmp_path):
    local, public = sources(tmp_path)
    excluded = hashlib.sha256(next((public / 'images').iterdir()).read_bytes()).hexdigest()
    output = tmp_path / 'review'
    prepare(local, public, output, count=2, exclude_hashes={excluded})
    rows = json.loads((output / 'public_50/annotations.json').read_text())
    assert excluded not in {row['source_sha256'] for row in rows}
    with pytest.raises(ValueError):
        prepare(local, public, tmp_path / 'not-enough', count=5)
    assert not (tmp_path / 'not-enough').exists()


def test_flags_discrepancies_without_silently_correcting_source(tmp_path):
    local, public = sources(tmp_path)
    path = local / 'outputs/photos_factures/labels_photos.json'
    rows = json.loads(path.read_text())
    for row in rows:
        row['total'] = 99
    path.write_text(json.dumps(rows))
    prepare(local, public, tmp_path / 'review', count=2)
    rows = json.loads((tmp_path / 'review/local_50/annotations.json').read_text())
    assert all('total_arithmetic_mismatch' in row['checks'] for row in rows)
    assert all('csv_json_mismatch:total' in row['checks'] for row in rows)
    assert all(row['source_reference_draft']['total'] == '99' for row in rows)


def test_rejects_source_paths_outside_the_selected_directory(tmp_path):
    local, public = sources(tmp_path)
    path = local / 'outputs/photos_factures/labels_photos.json'
    rows = json.loads(path.read_text())
    rows[0]['image'] = '../../../outside.jpg'
    path.write_text(json.dumps(rows))
    with pytest.raises(ValueError, match='unsafe source image'):
        prepare(local, public, tmp_path / 'review', count=2)
