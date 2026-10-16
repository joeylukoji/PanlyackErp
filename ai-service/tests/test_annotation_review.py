import hashlib
import io
import json
import re
import zipfile

import pytest
from fastapi.testclient import TestClient
from PIL import Image

from app.annotation_review import create_app
from app.evaluation import summarize


@pytest.fixture
def review(tmp_path):
    (tmp_path / 'images').mkdir()
    rows = []
    for index in range(2):
        identifier = f'invoice-{index + 1:03}'
        image = tmp_path / 'images' / f'{identifier}.png'
        Image.new('RGB', (30, 40), (index * 100, 0, 0)).save(image)
        rows.append({'id': identifier, 'image': f'images/{identifier}.png',
                     'source_file': image.name, 'source_sha256': hashlib.sha256(image.read_bytes()).hexdigest(),
                     'review_status': 'pending', 'reviewed_by': '', 'reviewed_at': '',
                     'reference': None, 'notes': '<script>alert(1)</script>'})
    (tmp_path / 'annotations.json').write_text(json.dumps(rows))
    predictions = tmp_path / 'predictions.json'
    predictions.write_text(json.dumps([{'id': 'invoice-001', 'status': 'ok', 'prediction': {
        'number': 'MODEL-NUMBER', 'partner': 'MODEL-SUPPLIER', 'issue_date': '2026-10-16',
        'currency': 'USD', 'total': '12.50', 'lines': []}}]))
    client = TestClient(create_app(tmp_path, predictions), base_url='http://127.0.0.1:8091')
    return tmp_path, client


def form(client, **changes):
    page = client.get('/?id=invoice-001').text
    values = {name: re.search(fr'name="{name}" value="([^"]*)"', page).group(1)
              for name in ('csrf', 'version', 'image_sha256')}
    values.update(action='confirm', reviewer='Local reviewer', document_type='invoice',
                  number='INV-1', partner='Supplier', issue_date='2026-10-16', currency='USD',
                  total='12.50', line_count='0', notes='Checked against the scan')
    values.update(changes)
    return values


def rows(root):
    return json.loads((root / 'annotations.json').read_text())


def test_page_is_read_only_and_does_not_promote_model_output(review):
    root, client = review
    before = (root / 'annotations.json').read_bytes()
    page = client.get('/').text
    assert 'hx-post=' in page
    assert 'value="MODEL-NUMBER"' not in page
    assert '&lt;script&gt;alert(1)&lt;/script&gt;' in page
    assert (root / 'annotations.json').read_bytes() == before
    assert client.get('/assets/htmx.min.js').status_code == 200
    assert client.get('/image/invoice-001?rotation=90').status_code == 200
    assert client.get('/image/unknown').status_code == 404


def test_confirmation_requires_csrf_same_origin_reviewer_and_invoice_type(review):
    root, client = review
    assert client.post('/review/invoice-001', data=form(client, csrf='bad')).status_code == 403
    assert client.post('/review/invoice-001', data=form(client, csrf='é')).status_code == 403
    assert client.post('/review/invoice-001', data=form(client), headers={'Origin': 'https://evil.example'}).status_code == 403
    assert client.get('/', headers={'Host': 'evil.example'}).status_code == 400
    assert client.post('/review/invoice-001', data=form(client, reviewer='')).status_code == 422
    assert client.post('/review/invoice-001', data=form(client, document_type='check')).status_code == 422
    assert rows(root)[0]['review_status'] == 'pending'


def test_confirmed_partial_reference_is_attributed_and_exported_without_invented_fields(review):
    root, client = review
    response = client.post('/review/invoice-001', data=form(client, currency='DEM'))
    assert response.status_code == 200
    row = rows(root)[0]
    assert row['review_status'] == 'confirmed' and row['document_type'] == 'invoice'
    assert row['reviewed_by'] == 'Local reviewer' and row['reviewed_at']
    assert row['reference']['currency'] == 'DEM' and row['reference']['total'] == '12.50'
    assert row['reference']['lines'] is None
    assert row['reviewed_image_sha256'] and row['review_history']
    archive = zipfile.ZipFile(io.BytesIO(client.get('/export').content))
    manifest = json.loads(archive.read('manifest.json'))
    pairs = [json.loads(line) for line in archive.read('reviewed-pairs.jsonl').splitlines()]
    assert manifest['exported_documents'] == 1
    assert pairs[0]['target_scope'] == 'partial_reference'
    assert pairs[0]['reference']['currency'] == 'DEM' and pairs[0]['reference']['lines'] is None
    assert len([name for name in archive.namelist() if name.startswith('images/')]) == 1


@pytest.mark.parametrize('changes', [{'issue_date': '2026-02-29'}, {'total': '1,20'}, {'total': 'NaN'}])
def test_invalid_date_or_decimal_does_not_change_annotations(review, changes):
    root, client = review
    assert client.post('/review/invoice-001', data=form(client, **changes)).status_code == 422
    assert rows(root)[0]['review_status'] == 'pending'


def test_exclusion_and_draft_are_not_scored_or_exported(review):
    root, client = review
    assert client.post('/review/invoice-001', data=form(client, action='draft')).status_code == 200
    assert rows(root)[0]['review_status'] == 'pending'
    assert client.post('/review/invoice-001', data=form(client, action='exclude', document_type='check', notes='Payment check')).status_code == 200
    row = rows(root)[0]
    assert row['review_status'] == 'excluded' and row['reference'] is None
    report = summarize(rows(root), [])
    assert report['excluded_documents'] == 1 and report['confirmed_documents'] == 0
    archive = zipfile.ZipFile(io.BytesIO(client.get('/export').content))
    assert json.loads(archive.read('manifest.json'))['exported_documents'] == 0


def test_stale_file_and_changed_image_cannot_overwrite_confirmation(review):
    root, client = review
    old_form = form(client)
    assert client.post('/review/invoice-001', data=form(client, notes='First review')).status_code == 200
    assert client.post('/review/invoice-001', data=old_form).status_code == 409
    assert rows(root)[0]['notes'] == 'First review'
    current_form = form(client)
    Image.new('RGB', (30, 40), 'blue').save(root / 'images/invoice-001.png')
    assert client.post('/review/invoice-001', data=current_form).status_code == 409
    archive = zipfile.ZipFile(io.BytesIO(client.get('/export').content))
    assert json.loads(archive.read('manifest.json'))['exported_documents'] == 0


def test_adding_a_line_preserves_the_form_without_writing_a_reference(review):
    root, client = review
    response = client.post('/review/invoice-001', data=form(client, action='add_line'))
    assert response.status_code == 200 and 'name="line_0_quantity"' in response.text
    assert rows(root)[0]['reference'] is None


def test_incomplete_lines_remain_drafts_and_saving_draft_revokes_confirmation(review):
    root, client = review
    line = {'line_count': '1', 'line_0_name': 'Item', 'line_0_quantity': '2.000001'}
    assert client.post('/review/invoice-001', data=form(client, **line)).status_code == 200
    row = rows(root)[0]
    assert row['reference']['lines'] is None
    assert row['draft_lines'][0]['quantity'] == '2.000001'
    assert client.post('/review/invoice-001', data=form(client, action='draft', line_items_complete='on', **line)).status_code == 200
    row = rows(root)[0]
    assert row['review_status'] == 'pending' and not row['reviewed_by']
    assert row['reference']['lines'][0]['quantity'] == '2.000001'
    assert summarize(rows(root), [])['confirmed_documents'] == 0


def test_form_limits_and_dataset_image_path_are_enforced(review):
    root, client = review
    assert client.post('/review/invoice-001', content='x' * 65537,
                       headers={'Content-Type': 'application/x-www-form-urlencoded'}).status_code == 413
    assert client.post('/review/invoice-001', json={}).status_code == 415
    data = rows(root)
    data[0]['image'] = '../outside.png'
    (root / 'annotations.json').write_text(json.dumps(data))
    assert client.get('/image/invoice-001').status_code == 404
