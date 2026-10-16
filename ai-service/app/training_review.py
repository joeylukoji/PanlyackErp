"""Prepare two review-only invoice lots from existing local files; never train or download."""
import argparse
import csv
import hashlib
import html
import json
import random
import re
import shutil
from collections import Counter, defaultdict
from datetime import datetime, timezone
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path

from PIL import Image, ImageOps


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def load(path):
    return json.loads(path.read_text(encoding='utf-8-sig'), parse_float=Decimal)


def write(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, default=str), encoding='utf-8')


def source_image(root, relative):
    path = (root / relative).resolve()
    if not path.is_relative_to(root.resolve()) or not path.is_file():
        raise ValueError(f'Missing or unsafe source image: {relative}')
    return path


def select(rows, count, seed):
    groups = defaultdict(list)
    seen = set()
    for row in rows:
        if row['sha256'] not in seen:
            groups[row['group']].append(row)
            seen.add(row['sha256'])
    if len(seen) < count:
        raise ValueError(f'Need {count} distinct documents; found {len(seen)}')
    rng = random.Random(seed)
    pools = [groups[key] for key in sorted(groups)]
    for pool in pools:
        rng.shuffle(pool)
    result = []
    while len(result) < count:
        for pool in pools:
            if pool and len(result) < count:
                result.append(pool.pop())
    return result


def local_draft(row):
    issue_date = datetime.strptime(row['date'], '%d/%m/%Y').date().isoformat()
    text = lambda key: str(row[key]) if row.get(key) is not None else None
    return {'number': row.get('invoice_no'), 'partner': row.get('store'), 'issue_date': issue_date,
            'currency': row.get('currency'), 'subtotal': text('subtotal'), 'discount': text('discount'),
            'tax_rate_fraction': text('tax_rate'), 'tax_rate_percent': str(Decimal(text('tax_rate')) * 100),
            'tax_total': text('tax'), 'total': text('total'), 'payment_method_source': row.get('payment'),
            'tendered_amount': text('paid'), 'change': text('change'),
            'lines': [{'name': line.get('name'), 'quantity': str(line['qty']),
                       'unit_price': str(line['unit_price']), 'line_total': str(line['line_total'])}
                      for line in row['items']]}


def local_checks(row, csv_row):
    checks = []
    amount = lambda key: Decimal(str(row[key]))
    if any(Decimal(str(line['qty'])) * Decimal(str(line['unit_price'])) != Decimal(str(line['line_total'])) for line in row['items']):
        checks.append('line_arithmetic_mismatch')
    if sum((Decimal(str(line['line_total'])) for line in row['items']), Decimal(0)) != amount('subtotal'):
        checks.append('subtotal_arithmetic_mismatch')
    net = amount('subtotal') - Decimal(str(row.get('discount', 0)))
    if net + amount('tax') != amount('total'):
        checks.append('total_arithmetic_mismatch')
    if (net * amount('tax_rate')).quantize(Decimal('.01'), rounding=ROUND_HALF_UP) != amount('tax'):
        checks.append('tax_arithmetic_mismatch')
    if amount('paid') - amount('change') != amount('total'):
        checks.append('payment_arithmetic_mismatch')
    if csv_row is None:
        return checks + ['missing_csv_row']
    for key in row.keys() & csv_row.keys():
        expected, actual = row[key], csv_row[key]
        if key == 'items':
            equal = expected == json.loads(actual, parse_float=Decimal)
        elif isinstance(expected, (int, Decimal)):
            equal = Decimal(str(expected)) == Decimal(actual)
        else:
            equal = (expected or '') == (actual or '')
        if not equal:
            checks.append(f'csv_json_mismatch:{key}')
    if 'nb_items' in csv_row and int(csv_row['nb_items']) != len(row['items']):
        checks.append('csv_json_mismatch:nb_items')
    return checks


def candidates(local_root, public_root, exclude_hashes):
    local_source = local_root / 'outputs/photos_factures'
    labels = load(local_source / 'labels_photos.json')
    with (local_source / 'labels_photos.csv').open(encoding='utf-8-sig', newline='') as stream:
        csv_rows = list(csv.DictReader(stream))
    if len({row['image'] for row in labels}) != len(labels) or len({row['image'] for row in csv_rows}) != len(csv_rows):
        raise ValueError('Source annotations must have unique image keys')
    csv_by_image = {row['image']: row for row in csv_rows}
    local = []
    for row in labels:
        path = source_image(local_source, row['image'])
        local.append({'path': path, 'sha256': digest(path), 'group': row['type'], 'raw': row,
                      'draft': local_draft(row), 'checks': local_checks(row, csv_by_image.get(row['image']))})
    config, = load(public_root / 'dataset_infos.json').values()
    names = config['features']['label']['names']
    shards = {}
    for shard in load(public_root / 'progress.json')['shards']:
        counts = shard['label_counts']
        if len(counts) != 1 or sum(counts.values()) != shard['rows']:
            raise ValueError('This adapter requires a verified single-class shard; read per-row labels otherwise')
        label = int(next(iter(counts)))
        if label < 0 or label >= len(names) or names[label] != 'invoice':
            raise ValueError('The public source must declare invoice-class documents')
        shards[Path(shard['file']).stem] = (shard, label)
    public = []
    for path in sorted((public_root / 'images').iterdir()):
        match = re.fullmatch(r'(train-\d{5}-of-\d{5})_(\d{5})\.(tif|tiff|png|jpg|jpeg)', path.name)
        if not match or match[1] not in shards:
            continue
        path = source_image(public_root, path.relative_to(public_root))
        sha = digest(path)
        if sha in exclude_hashes:
            continue
        shard, label = shards[match[1]]
        index = int(match[2])
        if index >= shard['rows']:
            raise ValueError('Image row falls outside its verified shard')
        with Image.open(path) as image:
            group = 'landscape' if image.width > image.height else 'portrait'
        public.append({'path': path, 'sha256': sha, 'group': group, 'raw': {
            'shard': shard['file'], 'row_index': index, 'label': label, 'label_name': names[label]},
            'draft': None, 'checks': ['classification_only_no_extraction_labels', 'actual_document_type_needs_human_review']})
    return local_source, local, public


def review_index(root, rows, title):
    cards = []
    esc = html.escape
    for row in rows:
        data = json.dumps(row['source_reference_draft'] or row['source_labels'], ensure_ascii=False, indent=2)
        cards.append(f'<article><h2>{esc(row["id"])}</h2><p>{esc(row["source_kind"])} · À vérifier</p>'
                     f'<a href="{row["image"]}" target="_blank"><img loading="lazy" src="{row["image"]}" alt="{row["id"]}"></a>'
                     f'<p><a href="labels/{row["id"]}.json">Labels et provenance JSON</a></p><pre>{esc(data)}</pre>'
                     f'<p>Contrôles : {esc(", ".join(row["checks"]) or "aucun écart arithmétique détecté")}</p></article>')
    (root / 'index.html').write_text('<!doctype html><html lang="fr"><meta charset="utf-8">'
        '<meta name="viewport" content="width=device-width,initial-scale=1">'
        f'<title>{esc(title)}</title><style>body{{font:16px system-ui;margin:24px;background:#f4f6f8;color:#203047}}'
        'main{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,420px),1fr));gap:24px}'
        'article{background:white;padding:20px;border:1px solid #d8e0e8}img{max-width:100%;height:auto;max-height:560px;object-fit:contain}'
        'pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:13px}a{color:#2052a0}</style>'
        f'<h1>{esc(title)}</h1><p>Comparez chaque image aux labels. Aucune référence n’est confirmée et aucun entraînement n’est lancé.</p>'
        '<p><a href="../index.html">Retour aux deux lots</a> · <a href="review.csv">Grille CSV</a></p><main>'
        + ''.join(cards) + '</main></html>', encoding='utf-8')


def prepare(local_root, public_root, output, count=50, seed=42, exclude_hashes=None):
    if output.exists():
        raise FileExistsError('Refusing to overwrite a review dataset')
    if count < 1:
        raise ValueError('Count must be positive')
    local_source, local, public = candidates(local_root, public_root, exclude_hashes or set())
    selected = [('local_50', select(local, count, seed), True), ('public_50', select(public, count, seed), False)]
    # Decode every chosen image before creating the output directory.
    for _, items, _ in selected:
        for item in items:
            with Image.open(item['path']) as image:
                image.load()
    output.mkdir(parents=True)
    manifests = []
    for name, items, is_local in selected:
        root = output / name
        for directory in ['images', 'labels', 'source', 'originals']:
            (root / directory).mkdir(parents=True)
        rows = []
        for index, item in enumerate(items, 1):
            identifier = f'{"local" if is_local else "public"}-{index:03}'
            image_name = f'images/{identifier}{".jpg" if is_local else ".png"}'
            if is_local:
                shutil.copyfile(item['path'], root / image_name)
            else:
                shutil.copyfile(item['path'], root / 'originals' / item['path'].name)
                with Image.open(item['path']) as original:
                    ImageOps.exif_transpose(original).convert('RGB').save(root / image_name)
            row = {'id': identifier, 'image': image_name, 'source_file': str(item['path']),
                   'source_sha256': item['sha256'], 'image_sha256': digest(root / image_name),
                   'source_kind': 'local_synthetic_receipt' if is_local else 'public_archival_scan',
                   'source_class_label': None if is_local else item['raw']['label_name'],
                   'source_labels': item['raw'], 'source_reference_draft': item['draft'],
                   'checks': item['checks'], 'document_type': 'unknown', 'review_status': 'pending',
                   'reviewed_by': '', 'reviewed_at': '', 'reference': None, 'notes': ''}
            write(root / 'labels' / f'{identifier}.json', row)
            rows.append(row)
        write(root / 'annotations.json', rows)
        write(root / 'predictions.json', [])
        with (root / 'review.csv').open('w', encoding='utf-8-sig', newline='') as stream:
            fields = ['id', 'image', 'source_kind', 'source_class_label', 'number', 'partner', 'issue_date',
                      'currency', 'total', 'checks', 'document_type', 'review_status', 'reviewed_by', 'notes']
            writer = csv.DictWriter(stream, fieldnames=fields)
            writer.writeheader()
            for row in rows:
                draft = row['source_reference_draft'] or {}
                writer.writerow({**{key: row.get(key, draft.get(key, '')) for key in fields}, 'checks': '; '.join(row['checks'])})
        for filename in (['labels_photos.json', 'labels_photos.csv'] if is_local else ['dataset_infos.json', 'progress.json', 'SOURCE_README.md', 'upstream-tree.json', 'README.md']):
            source = (local_source if is_local else public_root) / filename
            if source.is_file():
                shutil.copyfile(source, root / 'source' / filename)
        manifest = {'lot': name, 'documents': len(rows), 'synthetic': is_local, 'human_confirmed': 0,
                    'source_images_available': len(local) if is_local else len(public),
                    'category_distribution': dict(Counter(item['group'] for item in items)),
                    'selection': 'Seeded round-robin across local types or public image orientations',
                    'unused_source_files': [item['path'].name for item in (local if is_local else public) if item['sha256'] not in {row['source_sha256'] for row in rows}],
                    'limits': 'Source labels are not human-confirmed extraction references. No train/validation/test split is assigned.'}
        write(root / 'manifest.json', manifest)
        review_index(root, rows, '50 tickets locaux — labels fournis' if is_local else '50 scans publics — catégorie seule')
        manifests.append(manifest)
    write(output / 'manifest.json', {'created_at': datetime.now(timezone.utc).isoformat(), 'seed': seed,
                                   'documents': count * 2, 'lots': manifests, 'excluded_evaluation_hashes': len(exclude_hashes or set()),
                                   'training_started': False, 'training_ready': False})
    (output / 'index.html').write_text('<!doctype html><html lang="fr"><meta charset="utf-8"><meta name="viewport" content="width=device-width">'
        '<title>Factures à vérifier</title><h1>Deux lots à vérifier</h1>'
        f'<p>{count} documents locaux synthétiques et {count} scans publics. Aucune validation humaine ni entraînement.</p>'
        '<p><a href="local_50/index.html">Ouvrir le lot local et ses labels</a></p>'
        '<p><a href="public_50/index.html">Ouvrir le lot public et ses labels disponibles</a></p>'
        '<p>Les scans publics n’ont pas de montants ou de lignes annotés. Leurs champs restent à compléter.</p>'
        '<p>La grille CSV sert à la relecture ; ses modifications ne synchronisent pas automatiquement annotations.json.</p></html>', encoding='utf-8')
    return manifests


def verify_parquet(output, public_root):
    import pyarrow.parquet as pq
    rows = load(output / 'public_50/annotations.json')
    by_shard = defaultdict(dict)
    for row in rows:
        source = row['source_labels']
        by_shard[source['shard']][source['row_index']] = row
    verified = 0
    hashes = {}
    tree = {Path(item['path']).name: item for item in load(public_root / 'upstream-tree.json') if item['path'].endswith('.parquet')}
    for shard, wanted in by_shard.items():
        path = public_root / shard
        hashes[shard] = digest(path)
        if hashes[shard] != tree[shard]['lfs']['oid']:
            raise ValueError('Parquet does not match pinned publisher metadata')
        index = 0
        for batch in pq.ParquetFile(path).iter_batches(batch_size=32):
            for raw in batch.to_pylist():
                if index in wanted:
                    row = wanted[index]
                    if raw['label'] != row['source_labels']['label'] or hashlib.sha256(raw['image']['bytes']).hexdigest() != row['source_sha256']:
                        raise ValueError('Selected image does not match original Parquet row/label')
                    verified += 1
                index += 1
    if verified != len(rows):
        raise ValueError('Not every selected row was verified')
    write(output / 'public_50/parquet-verification.json', {'verified_rows': verified, 'publisher_lfs_sha256': hashes})


def write_guides(output, public_root):
    data_root = public_root.parent
    catalog = [
        {'name': 'RVL-CDIP invoice subset', 'kind': 'real_archival_scans', 'source': 'https://huggingface.co/datasets/chainyo/rvl-cdip-invoice',
         'revision': 'fad615c9ceaecb4476b0a01f29c0a15b276b3a2b', 'images_in_this_pack': len(load(output / 'public_50/annotations.json')),
         'labels': ['document class: invoice (mirror encoding 6)'], 'missing': ['invoice fields', 'line items', 'accounting postings'],
         'rights': 'Other; see copied source card and UCSF rights notice. No blanket training permission asserted.'},
    ]
    support = output / 'public_datasets'
    support.mkdir(exist_ok=True)
    sources = [
        ('invoice-ocr-synthetic', ['ground_truth_released.json', 'selected_images.json', 'SOURCE_README.md', 'LICENSE-CC-BY-4.0.txt', 'REVISION.txt'],
         'synthetic_invoices_receipts', ['supplier/customer fields', 'document dates and amounts', 'line item annotations'],
         'Labels for fictional rendered documents, not real commercial transactions. No additional images copied into the 100-document pack.'),
        ('real-accounting/iowa-checkbook', ['ledger_first_10000_rows.csv', 'columns.json', 'README.md', 'LICENSE-CC-BY-4.0.txt', 'sample_verification.json'],
         'real_public_payments', ['22 exported columns including invoice_no, vendor_name, amount, dates and expense_category'],
         'Public payments only: no balanced debit/credit targets or matched invoice images. Partial ZIP archives are excluded.'),
        ('beancount-us', ['example.beancount', 'starter.beancount', 'COPYING', 'REVISION.txt'],
         'fictional_double_entry_examples', ['dated transaction directives and account postings'],
         'Educational generated ledgers, not real company books or invoice-to-journal labels.'),
    ]
    for name, files, kind, labels, limits in sources:
        source = data_root / name
        if not source.is_dir():
            continue
        destination = support / Path(name).name
        destination.mkdir(exist_ok=True)
        copied = []
        for filename in files:
            if (source / filename).is_file():
                shutil.copyfile(source / filename, destination / filename)
                copied.append({'file': str((destination / filename).relative_to(output)).replace('\\', '/'), 'sha256': digest(source / filename)})
        entry = {'name': name, 'kind': kind, 'labels': labels, 'limits': limits, 'source_directory': str(source.resolve()), 'files': copied}
        if name == 'invoice-ocr-synthetic':
            entry['labelled_records'] = len(load(source / 'ground_truth_released.json'))
        if name.endswith('iowa-checkbook'):
            with (source / 'ledger_first_10000_rows.csv').open(encoding='utf-8-sig', newline='') as stream:
                entry['payment_rows'] = sum(1 for _ in csv.DictReader(stream))
        catalog.append(entry)
    write(support / 'catalog.json', catalog)
    (output / 'DATA_DICTIONARY.md').write_text('''# Data dictionary

Every document has an image, a per-document JSON label file, and a row in `review.csv`.

| Field | Meaning |
| --- | --- |
| id / image | Stable local ID and canonical image path within its lot |
| source_file / source_sha256 | Original input path and exact original image byte hash |
| image_sha256 | Hash of the canonical copied JPEG or rendered PNG |
| source_kind | Local synthetic receipt or real public archival scan |
| source_labels | Supplied annotation row, never an assistant or model prediction |
| source_class_label | Public dataset category only; not proof of actual document type |
| source_reference_draft | Local source fields normalized for inspection; not human confirmed |
| checks | Detected arithmetic/CSV discrepancies or missing field-label warnings |
| document_type | Unknown until a human classifies the actual scan |
| review_status / reviewed_by | Pending and blank initially; never automatically confirmed |
| reference | Null until the operator records reviewed extraction fields |

Local normalization maps `invoice_no` to `number`, `store` to `partner`, and `date` from DD/MM/YYYY to ISO `issue_date`. Amounts and quantities are decimal strings. `tax_rate_fraction` preserves 0.16; `tax_rate_percent` expresses 16 percent. Discounts are preserved, not silently allocated across lines. `paid` is the tendered amount and `change` is returned change: tendered cash is not automatically an ERP payment. No SKU, accounting account, missing unit or per-line tax rate is invented.

The complete original local CSV/JSON contain 58 source rows. The lot selects 50 of them; use the selected IDs and source image paths to join. The four root-level examples and their older label files were not selected; the source is `outputs/photos_factures`. Per-document JSON uses decimal strings for decimal values; the full original source files remain byte-identical snapshots.

RVL-CDIP supplies a class label only. Its selected original TIFF files are retained under `public_50/originals`; canonical PNGs are EXIF-oriented RGB previews. Field references stay null. Checks or budgets can be misclassified in this archive, so actual invoice status needs human review.

`public_datasets/catalog.json` documents the other already-downloaded public sources and their available labels. These supporting CSV/JSON/ledger files are not additional invoice-image training pairs. Source cards, revision files and available licenses are copied with them. Their licenses differ; do not treat them as one newly licensed dataset.
''', encoding='utf-8')
    index = output / 'index.html'
    content = index.read_text(encoding='utf-8')
    links = '<p><a href="DATA_DICTIONARY.md">Dictionnaire des labels</a> · <a href="public_datasets/catalog.json">Autres datasets publics et labels disponibles</a> · <a href="README.md">Procédure de vérification</a></p>'
    if links not in content:
        index.write_text(content.replace('</html>', links + '</html>'), encoding='utf-8')
    (output / 'README.md').write_text('''# Invoice training preparation: human review first

Open `index.html` to inspect the two separate lots. The package contains 50 local synthetic receipts with supplied field labels and 50 real public archival scans with classification labels only. These are 100 distinct documents, not 100 verified invoices or 100 complete extraction targets.

Review each image against its labels, classify the actual document, correct ambiguous fields and record a named human confirmation before using extraction targets. All records start pending. Use `review.csv` as an inspection worksheet; editing it does not automatically synchronize `annotations.json`. Keep null for unknown fields. The included `predictions.json` files are intentionally empty: no inference ran.

The existing annotation workbench can open either lot with `scripts/review-invoices.ps1 -Dataset <absolute lot path> -Predictions <absolute lot path>/predictions.json -Port 8094`. It requires manually recording references; supplied source drafts do not become confirmed references automatically. Do not edit individual label snapshots as a substitute for updating the authoritative annotations file.

Selection is deterministic (seed 42), balanced across available local ticket categories and public orientations. Previously used evaluation image hashes are excluded from the public lot. Exact-byte exclusion does not establish supplier/layout independence. No training/validation/test split or model training is performed. The package is not ready-to-run SFT input; public fields are missing, human review is pending, and source usage rights still need to match the intended use.

`manifest.json`, the lot manifests, `verification.json` and `SHA256SUMS.txt` describe selection and checks. When `--verify-parquet` is requested, `public_50/parquet-verification.json` records original-row checks. Originals were copied, never modified. No interrupted dataset download was resumed. Supporting public label files and dataset limitations are described in `DATA_DICTIONARY.md` and `public_datasets/catalog.json`.
''', encoding='utf-8')


def finalize(output):
    rows = []
    for lot in ['local_50', 'public_50']:
        root = output / lot
        current = load(root / 'annotations.json')
        for row in current:
            if digest(root / row['image']) != row['image_sha256']:
                raise ValueError('Canonical image hash mismatch')
            with Image.open(root / row['image']) as image:
                image.load()
            original = root / row['image'] if lot == 'local_50' else root / 'originals' / Path(row['source_file']).name
            if digest(original) != row['source_sha256']:
                raise ValueError('Original image hash mismatch')
            if row['review_status'] != 'pending' or row['reference'] is not None:
                raise ValueError('Unexpected approved reference')
        rows.extend(current)
    if len({row['source_sha256'] for row in rows}) != len(rows):
        raise ValueError('Duplicate original image across lots')
    write(output / 'verification.json', {'documents': len(rows), 'canonical_images_decoded': len(rows),
        'original_hashes_checked': len(rows), 'distinct_original_hashes': len(rows), 'human_confirmed': 0,
        'local_arithmetic_or_csv_issue_documents': sum(bool(row['checks']) for row in rows if row['source_kind'] == 'local_synthetic_receipt'),
        'public_field_annotations_available': 0, 'public_parquet_rows_verified': (
            load(output / 'public_50/parquet-verification.json')['verified_rows']
            if (output / 'public_50/parquet-verification.json').exists() else None), 'training_started': False})
    files = sorted(path for path in output.rglob('*') if path.is_file() and path.name != 'SHA256SUMS.txt')
    (output / 'SHA256SUMS.txt').write_text(''.join(f'{digest(path)}  {path.relative_to(output).as_posix()}\n' for path in files), encoding='utf-8')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--local', type=Path, required=True)
    parser.add_argument('--public', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--exclude-evaluations', type=Path, required=True)
    parser.add_argument('--verify-parquet', action='store_true')
    args = parser.parse_args()
    excluded = set()
    for path in args.exclude_evaluations.glob('*/annotations.json'):
        excluded.update(row['source_sha256'] for row in load(path) if row.get('source_sha256'))
    prepare(args.local.resolve(), args.public.resolve(), args.output.resolve(), exclude_hashes=excluded)
    if args.verify_parquet:
        verify_parquet(args.output, args.public)
    write_guides(args.output, args.public)
    finalize(args.output)
    print(f'Prepared 100 pending documents in {args.output}; no training started.')


if __name__ == '__main__':
    main()
