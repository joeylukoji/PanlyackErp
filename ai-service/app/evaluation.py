"""Local invoice benchmark; never imports ERP data or approves model predictions."""
import argparse
import asyncio
import base64
import hashlib
import html
import json
import random
import time
from collections import defaultdict
from datetime import datetime, timezone
from decimal import Decimal, InvalidOperation
from pathlib import Path

from PIL import Image, ImageOps

from app.extractors import OllamaExtractor, validate_file
from app.schemas import ExtractionRequest

DECIMAL_FIELDS = {'total', 'quantity', 'unit_price', 'tax_rate'}


def save_json(path: Path, value) -> None:
    temporary = path.with_suffix(path.suffix + '.tmp')
    temporary.write_text(json.dumps(value, indent=2, ensure_ascii=False), encoding='utf-8')
    temporary.replace(path)


def prepare(source: Path, output: Path, count: int = 50, seed: int = 42) -> list[dict]:
    if output.exists():
        raise FileExistsError('Refusing to overwrite an existing annotation dataset')
    if count < 1:
        raise ValueError('Count must be positive')
    groups = defaultdict(list)
    seen = set()
    for path in sorted(source.iterdir()):
        if path.suffix.lower() not in {'.png', '.jpg', '.jpeg', '.tif', '.tiff'}:
            continue
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        if digest in seen:
            continue
        seen.add(digest)
        with Image.open(path) as image:
            width, height = image.size
        shape = 'landscape' if width > height else 'portrait'
        groups[(shape, path.stat().st_size // 50000)].append((path, digest))
    if len(seen) < count:
        raise ValueError(f'Need {count} distinct source images; found {len(seen)}')
    rng = random.Random(seed)
    pools = [groups[key] for key in sorted(groups)]
    for pool in pools:
        rng.shuffle(pool)
    selected = []
    while len(selected) < count:
        for pool in pools:
            if pool and len(selected) < count:
                selected.append(pool.pop())
    (output / 'images').mkdir(parents=True)
    annotations = []
    cards = []
    for index, (path, digest) in enumerate(selected, 1):
        identifier = f'invoice-{index:03}'
        relative_image = f'images/{identifier}.png'
        with Image.open(path) as original:
            image = ImageOps.exif_transpose(original).convert('RGB')
            image.save(output / relative_image)
            width, height = image.size
        annotations.append({
            'id': identifier, 'image': relative_image, 'source_file': path.name,
            'source_sha256': digest, 'width': width, 'height': height,
            'review_status': 'pending', 'reviewed_by': '', 'reviewed_at': '',
            'reference': None, 'notes': '',
        })
        cards.append(f'<article><h2>{identifier}</h2><p>{html.escape(path.name)}</p>'
                     f'<a href="{relative_image}"><img loading="lazy" src="{relative_image}" alt="{identifier}"></a></article>')
    save_json(output / 'annotations.json', annotations)
    save_json(output / 'selection.json', {
        'created_at': datetime.now(timezone.utc).isoformat(), 'seed': seed, 'count': count,
        'available_distinct_images': len(seen), 'source': str(source.resolve()),
        'selection': 'Round-robin across orientation and source-file-size buckets; seeded within buckets',
        'limits': 'Diversity proxy only; not supplier-stratified or representative of all invoices',
    })
    save_json(output / 'reference-template.json', {
        'number': None, 'partner': None, 'issue_date': None, 'currency': None, 'total': None,
        'lines': [{'name': None, 'quantity': None, 'unit_price': None, 'tax_rate': None}],
    })
    (output / 'index.html').write_text(
        '<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width">'
        '<title>Invoice benchmark review</title><style>body{font:16px system-ui;margin:2rem;background:#f1f3f5}'
        'main{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,320px),1fr));gap:1rem}'
        'article{background:white;padding:1rem}img{width:100%;height:auto}h2{font-size:1rem}</style>'
        '<h1>Invoice benchmark: manual review</h1><p>Open a scan at full size. Record only visible facts in '
        'annotations.json using reference-template.json. Null means unknown. Predictions are not ground truth.</p>'
        '<main>' + ''.join(cards) + '</main></html>', encoding='utf-8')
    return annotations


def reference_fields(reference: dict, prefix: str = ''):
    for key, value in reference.items():
        name = f'{prefix}{key}'
        if value is None:
            continue
        if key == 'lines':
            yield 'lines_count', len(value)
            for index, line in enumerate(value):
                yield from reference_fields(line, f'lines.{index}.')
        else:
            yield name, value


def predicted_field(prediction: dict | None, name: str):
    if prediction is None:
        return None
    if name == 'lines_count':
        return len(prediction['lines']) if 'lines' in prediction else None
    value = prediction
    try:
        for part in name.split('.'):
            value = value[int(part)] if isinstance(value, list) else value[part]
        return value
    except (KeyError, IndexError, TypeError):
        return None


def matches(name: str, expected, actual) -> bool:
    if actual is None:
        return False
    if name.split('.')[-1] in DECIMAL_FIELDS:
        try:
            left, right = Decimal(str(expected)), Decimal(str(actual))
            return left.is_finite() and right.is_finite() and left == right
        except InvalidOperation:
            return False
    return ' '.join(str(expected).split()).casefold() == ' '.join(str(actual).split()).casefold()


def summarize(annotations: list[dict], predictions: list[dict]) -> dict:
    confirmed = [row for row in annotations if row.get('review_status') == 'confirmed'
                 and row.get('reviewed_by', '').strip() and isinstance(row.get('reference'), dict)
                 and row['reference']]
    by_id = {row['id']: row for row in predictions}
    fields = defaultdict(lambda: {'matched': 0, 'scored': 0})
    for row in confirmed:
        # Missing/failed predictions count as mismatches, rather than disappearing from accuracy.
        result = by_id.get(row['id'], {})
        prediction = result.get('prediction') if result.get('status') == 'ok' else None
        for name, expected in reference_fields(row['reference']):
            metric = fields[name]
            metric['scored'] += 1
            metric['matched'] += int(matches(name, expected, predicted_field(prediction, name)))
    scored = sum(metric['scored'] for metric in fields.values())
    matched = sum(metric['matched'] for metric in fields.values())
    return {
        'dataset_documents': len(annotations), 'attempted_documents': len(predictions),
        'successful_extractions': sum(row['status'] == 'ok' for row in predictions),
        'failed_extractions': sum(row['status'] == 'error' for row in predictions),
        'confirmed_documents': len(confirmed), 'pending_documents': len(annotations) - len(confirmed),
        'scored_fields': scored, 'matched_fields': matched,
        'field_accuracy': matched / scored if scored else None,
        'fields': dict(fields),
        'limits': 'Schema validity is not accuracy. Only human-confirmed, non-null fields are scored. '
                  'Line items are compared in document order. No ERP data is posted.',
    }


async def run(dataset: Path, output: Path, extractor, limit: int | None = None) -> dict:
    annotations = json.loads((dataset / 'annotations.json').read_text(encoding='utf-8'), parse_float=Decimal)
    if not annotations:
        raise ValueError('The evaluation dataset must contain documents')
    output.mkdir(parents=True, exist_ok=False)
    predictions = []
    for row in annotations[:limit]:
        started = time.monotonic()
        result = {'id': row['id']}
        try:
            path = (dataset / row['image']).resolve()
            if not path.is_relative_to(dataset.resolve()):
                raise ValueError('Image must remain inside the evaluation dataset')
            request = ExtractionRequest(mime_type='image/png', file_base64=base64.b64encode(path.read_bytes()).decode())
            validate_file(request)
            invoice = await extractor.extract(request)
            result.update(status='ok', prediction=invoice.model_dump())
        except Exception as exc:
            # Preserve failures in the denominator and continue the local benchmark.
            result.update(status='error', error_type=type(exc).__name__, error=str(exc)[:1000])
        result['elapsed_seconds'] = round(time.monotonic() - started, 3)
        predictions.append(result)
        save_json(output / 'predictions.json', predictions)
        report = summarize(annotations, predictions)
        report.update(model=getattr(extractor, 'model', 'test'), base_url=getattr(extractor, 'base_url', ''),
                      updated_at=datetime.now(timezone.utc).isoformat())
        save_json(output / 'report.json', report)
        print(f"{row['id']}: {result['status']} ({result['elapsed_seconds']}s)", flush=True)
    return report


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    selection = sub.add_parser('prepare')
    selection.add_argument('--source', type=Path, required=True)
    selection.add_argument('--output', type=Path, required=True)
    selection.add_argument('--count', type=int, default=50)
    selection.add_argument('--seed', type=int, default=42)
    benchmark = sub.add_parser('run')
    benchmark.add_argument('--dataset', type=Path, required=True)
    benchmark.add_argument('--output', type=Path, required=True)
    benchmark.add_argument('--model', default='gemma3:4b')
    benchmark.add_argument('--base-url', default='http://127.0.0.1:11434')
    benchmark.add_argument('--limit', type=int)
    scoring = sub.add_parser('score')
    scoring.add_argument('--dataset', type=Path, required=True)
    scoring.add_argument('--predictions', type=Path, required=True)
    args = parser.parse_args()
    if args.command == 'prepare':
        rows = prepare(args.source, args.output, args.count, args.seed)
        print(f'Prepared {len(rows)} documents for human annotation in {args.output}')
    elif args.command == 'run':
        if args.limit is not None and args.limit < 1:
            parser.error('--limit must be positive')
        asyncio.run(run(args.dataset, args.output, OllamaExtractor(args.base_url, args.model), args.limit))
    else:
        annotations = json.loads((args.dataset / 'annotations.json').read_text(encoding='utf-8'), parse_float=Decimal)
        predictions = json.loads(args.predictions.read_text(encoding='utf-8'), parse_float=Decimal)
        print(json.dumps(summarize(annotations, predictions), indent=2))


if __name__ == '__main__':
    main()
