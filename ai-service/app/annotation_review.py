"""Single-operator, loopback-only workbench for human reference annotations."""
import argparse
import hashlib
import io
import json
import secrets
import threading
import zipfile
from datetime import date, datetime, timezone
from decimal import Decimal
from pathlib import Path
from urllib.parse import parse_qs, quote, unquote

import uvicorn
from fastapi import FastAPI, HTTPException, Request
from fastapi.responses import FileResponse, HTMLResponse, Response
from PIL import Image
from pydantic import BaseModel, ConfigDict, Field, ValidationError, field_validator

from app.evaluation import save_json, summarize
from app.review_html import render_page, render_preview
from app.schemas import DecimalString

TYPES = {'unknown', 'invoice', 'receipt', 'check', 'purchase_order', 'pro_forma', 'other', 'unreadable'}
ASSETS = Path(__file__).resolve().parents[2] / 'web' / 'static'


class ReferenceLine(BaseModel):
    model_config = ConfigDict(extra='forbid')
    name: str | None = Field(default=None, max_length=200)
    quantity: DecimalString | None = None
    unit_price: DecimalString | None = None
    tax_rate: DecimalString | None = None
    unit: str | None = Field(default=None, max_length=20)


class Reference(BaseModel):
    model_config = ConfigDict(extra='forbid')
    number: str | None = Field(default=None, max_length=100)
    partner: str | None = Field(default=None, max_length=200)
    issue_date: str | None = None
    currency: str | None = Field(default=None, pattern=r'^[A-Z]{3}$')
    total: DecimalString | None = None
    lines: list[ReferenceLine] | None = Field(default=None, max_length=100)

    @field_validator('issue_date')
    @classmethod
    def calendar_date(cls, value):
        if value is not None and date.fromisoformat(value).isoformat() != value:
            raise ValueError('Date must use YYYY-MM-DD')
        return value


def one(form: dict, key: str, default: str = '') -> str:
    return form.get(key, default).strip()


def image_path(dataset: Path, row: dict) -> Path:
    path = (dataset / row['image']).resolve()
    if not path.is_relative_to(dataset.resolve()) or path.suffix.lower() != '.png' or not path.is_file():
        raise HTTPException(404, 'Image indisponible')
    return path


def image_digest(dataset: Path, row: dict) -> str:
    return hashlib.sha256(image_path(dataset, row).read_bytes()).hexdigest()


def parse_reference(form: dict) -> tuple[dict, list[dict]]:
    count = int(one(form, 'line_count', '0'))
    if not 0 <= count <= 100:
        raise ValueError('Invalid line count')
    lines = []
    for index in range(count):
        values = {key: one(form, f'line_{index}_{key}') or None for key in ReferenceLine.model_fields}
        if any(value is not None for value in values.values()):
            lines.append(ReferenceLine.model_validate(values).model_dump())
    values = {key: one(form, key) or None for key in Reference.model_fields if key != 'lines'}
    if values['currency']:
        values['currency'] = values['currency'].upper()
    values['lines'] = lines if form.get('line_items_complete') == 'on' else None
    return Reference.model_validate(values).model_dump(), lines


def has_reference(reference: dict) -> bool:
    return any(value is not None and value != [] for value in reference.values())


def export_archive(dataset: Path, rows: list[dict]) -> bytes:
    output = io.BytesIO()
    pairs, skipped = [], []
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as archive:
        for row in rows:
            if (row.get('review_status') != 'confirmed' or row.get('document_type') != 'invoice'
                    or not row.get('reviewed_by') or not row.get('reviewed_at')):
                skipped.append({'id': row['id'], 'reason': 'Not a human-confirmed invoice reference'})
                continue
            try:
                reference = Reference.model_validate(row.get('reference')).model_dump()
                image_bytes = image_path(dataset, row).read_bytes()
                digest = hashlib.sha256(image_bytes).hexdigest()
                if not has_reference(reference) or digest != row.get('reviewed_image_sha256'):
                    raise ValueError('Image changed or reference is empty')
            except (ValueError, HTTPException):
                skipped.append({'id': row['id'], 'reason': 'Reference or reviewed image needs revalidation'})
                continue
            name = f"images/document-{len(pairs) + 1:04}.png"
            archive.writestr(name, image_bytes)
            pairs.append({'id': row['id'], 'image': name, 'image_sha256': digest,
                          'original_source_sha256': row.get('source_sha256'),
                          'target_scope': 'partial_reference', 'reference': reference,
                          'reviewed_by': row['reviewed_by'], 'reviewed_at': row['reviewed_at']})
        archive.writestr('reviewed-pairs.jsonl', ''.join(json.dumps(pair, ensure_ascii=False) + '\n' for pair in pairs))
        archive.writestr('manifest.json', json.dumps({
            'created_at': datetime.now(timezone.utc).isoformat(), 'exported_documents': len(pairs),
            'skipped': skipped, 'purpose': 'Human-reviewed extraction reference pairs, with unknown fields preserved.',
            'limits': 'Not a ready-to-run full Invoice SFT corpus. Source rights still apply. No training has started.',
        }, indent=2))
    return output.getvalue()


def create_app(dataset: Path, predictions_file: Path) -> FastAPI:
    dataset = dataset.resolve()
    annotations_file = dataset / 'annotations.json'
    predictions = json.loads(predictions_file.read_text(encoding='utf-8'), parse_float=Decimal)
    by_id = {row['id']: row for row in predictions}
    csrf = secrets.token_urlsafe(32)
    lock = threading.RLock()
    app = FastAPI(title='PanlyackERP annotation review', docs_url=None, redoc_url=None, openapi_url=None)

    def snapshot():
        raw = annotations_file.read_bytes()
        rows = json.loads(raw, parse_float=Decimal)
        if not rows or len({row['id'] for row in rows}) != len(rows):
            raise ValueError('Dataset must have unique document identifiers')
        return rows, hashlib.sha256(raw).hexdigest()

    snapshot()

    def lookup(rows, identifier):
        row = next((item for item in rows if item['id'] == identifier), None)
        if row is None:
            raise HTTPException(404, 'Document introuvable')
        return row

    @app.middleware('http')
    async def local_only(request: Request, call_next):
        if request.url.hostname not in {'127.0.0.1', 'localhost'}:
            response = Response('Local access only', status_code=400)
        elif request.method == 'POST' and request.headers.get('origin') not in {None, str(request.base_url).rstrip('/')}:
            response = Response('Origin refused', status_code=403)
        else:
            response = await call_next(request)
        response.headers.update({
            'Content-Security-Policy': "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; frame-ancestors 'none'; form-action 'self'",
            'X-Content-Type-Options': 'nosniff', 'Referrer-Policy': 'no-referrer', 'Cache-Control': 'no-store',
        })
        return response

    def page(request, identifier=None, form=None, message='', status=200):
        rows, version = snapshot()
        row = lookup(rows, identifier) if identifier else rows[0]
        digest = image_digest(dataset, row)
        content = render_page(rows, row, by_id.get(row['id']), summarize(rows, predictions),
                              version, digest, csrf, form, message,
                              unquote(request.cookies.get('reviewer', '')), request.headers.get('HX-Request') == 'true',
                              request.query_params.get('filter', 'all'))
        return HTMLResponse(content, status_code=status)

    @app.get('/')
    def home(request: Request, id: str | None = None):
        with lock:
            return page(request, id)

    @app.get('/assets/{name}')
    def asset(name: str):
        files = {'htmx.min.js': ASSETS / 'htmx.min.js', 'logo.png': ASSETS / 'panlyack-logo.png',
                 'review.css': Path(__file__).with_name('review.css'), 'review.js': Path(__file__).with_name('review.js')}
        if name not in files:
            raise HTTPException(404)
        return FileResponse(files[name])

    @app.get('/image/{identifier}')
    def image(identifier: str, rotation: int = 0):
        if rotation not in {0, 90, 180, 270}:
            raise HTTPException(422, 'Rotation invalide')
        with lock:
            row = lookup(snapshot()[0], identifier)
            path = image_path(dataset, row)
            if rotation == 0:
                return FileResponse(path, media_type='image/png')
            with Image.open(path) as source:
                output = io.BytesIO()
                source.rotate(-rotation, expand=True).save(output, format='PNG')
            return Response(output.getvalue(), media_type='image/png')

    @app.get('/preview/{identifier}')
    def preview(identifier: str, rotation: int = 0):
        lookup(snapshot()[0], identifier)
        if rotation not in {0, 90, 180, 270}:
            raise HTTPException(422)
        return HTMLResponse(render_preview(identifier, rotation))

    @app.post('/review/{identifier}')
    async def review(request: Request, identifier: str):
        if request.headers.get('content-type', '').split(';')[0] != 'application/x-www-form-urlencoded':
            raise HTTPException(415)
        raw = bytearray()
        async for chunk in request.stream():
            raw.extend(chunk)
            if len(raw) > 65536:
                raise HTTPException(413, 'Formulaire trop volumineux')
        try:
            fields = parse_qs(raw.decode(), keep_blank_values=True, max_num_fields=1000)
            if any(len(values) != 1 for values in fields.values()):
                raise ValueError('Duplicate fields')
            form = {key: values[0] for key, values in fields.items()}
        except (ValueError, UnicodeError):
            raise HTTPException(422, 'Formulaire invalide')
        if not secrets.compare_digest(form.get('csrf', '').encode('utf-8'), csrf.encode('ascii')):
            raise HTTPException(403, 'Recharge la page avant de réessayer')
        with lock:
            rows, version = snapshot()
            row = lookup(rows, identifier)
            if form.get('version') != version or form.get('image_sha256') != image_digest(dataset, row):
                return page(request, identifier, form, 'Le document ou les annotations ont changé. Recharge la page avant de continuer.', 409)
            action = form.get('action')
            if action == 'add_line':
                try:
                    count = int(form.get('line_count', '0'))
                    if not 0 <= count < 100:
                        raise ValueError()
                    form['line_count'] = str(count + 1)
                except ValueError:
                    return page(request, identifier, form, 'La limite est de 100 lignes.', 422)
                return page(request, identifier, form, 'Ligne ajoutée. Le brouillon reste à enregistrer.')
            reviewer = one(form, 'reviewer')
            kind = one(form, 'document_type')
            notes = one(form, 'notes')
            if action not in {'draft', 'confirm', 'exclude'} or kind not in TYPES or len(reviewer) > 80 or len(notes) > 2000:
                return page(request, identifier, form, 'Vérifie le type de document, le nom et les notes.', 422)
            if action in {'confirm', 'exclude'} and not reviewer:
                return page(request, identifier, form, 'Indique ton nom pour attribuer cette revue.', 422)
            if action == 'confirm' and kind != 'invoice':
                return page(request, identifier, form, 'Seules les factures peuvent être confirmées comme références.', 422)
            if action == 'exclude' and (kind in {'invoice', 'unknown'} or not notes):
                return page(request, identifier, form, 'Choisis un autre type et précise le motif pour écarter ce document.', 422)
            try:
                reference, draft_lines = (None, []) if action == 'exclude' else parse_reference(form)
                if action == 'confirm' and not has_reference(reference):
                    raise ValueError('Empty reference')
            except (ValueError, ValidationError):
                return page(request, identifier, form, 'Vérifie les champs : date AAAA-MM-JJ, devise à 3 lettres et nombres avec un point (ex. 12.50). Confirme au moins un champ lisible.', 422)
            timestamp = datetime.now(timezone.utc).isoformat()
            previous = {key: row.get(key) for key in ('review_status', 'reviewed_by', 'reviewed_at', 'document_type', 'reference', 'notes', 'draft_lines')}
            row.setdefault('review_history', []).append({'at': timestamp, 'by': reviewer, 'action': action, 'previous': previous})
            row.update(document_type=kind, reference=reference, draft_lines=draft_lines,
                       line_items_complete=form.get('line_items_complete') == 'on', notes=notes,
                       review_status={'draft': 'pending', 'confirm': 'confirmed', 'exclude': 'excluded'}[action],
                       reviewed_by=reviewer if action != 'draft' else '', reviewed_at=timestamp if action != 'draft' else '',
                       reviewed_image_sha256=image_digest(dataset, row) if action == 'confirm' else '')
            save_json(annotations_file, rows)
            response = page(request, identifier, message={'draft': 'Brouillon enregistré.', 'confirm': 'Référence confirmée. Aucun entraînement ni écriture comptable lancé.', 'exclude': 'Document écarté du lot de factures.'}[action])
            response.headers['X-Review-Saved'] = 'true'
            if reviewer:
                response.set_cookie('reviewer', quote(reviewer), httponly=True, samesite='strict', max_age=28800)
            return response

    @app.get('/export')
    def export():
        with lock:
            archive = export_archive(dataset, snapshot()[0])
        return Response(archive, media_type='application/zip', headers={'Content-Disposition': 'attachment; filename="reviewed-invoice-references.zip"'})

    return app


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--dataset', type=Path, required=True)
    parser.add_argument('--predictions', type=Path, required=True)
    parser.add_argument('--port', type=int, default=8093)
    args = parser.parse_args()
    uvicorn.run(create_app(args.dataset, args.predictions), host='127.0.0.1', port=args.port, proxy_headers=False)


if __name__ == '__main__':
    main()
