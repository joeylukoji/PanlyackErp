"""Escaped, server-rendered HTML for the local annotation workbench."""
import html
from urllib.parse import quote

LABELS = {'pending': 'À revoir', 'confirmed': 'Confirmée', 'excluded': 'Écarté'}
TYPES = {'unknown': 'À préciser', 'invoice': 'Facture', 'receipt': 'Reçu / ticket',
         'check': 'Chèque / paiement', 'purchase_order': 'Commande / demande d’achat',
         'pro_forma': 'Facture pro forma', 'other': 'Autre document', 'unreadable': 'Scan illisible'}
HEADERS = {'number': 'Numéro de facture', 'partner': 'Fournisseur', 'issue_date': 'Date de facture',
           'currency': 'Code de devise', 'total': 'Total du document'}
LINE_FIELDS = {'name': 'Désignation', 'quantity': 'Quantité', 'unit_price': 'Prix unitaire',
               'tax_rate': 'Taxe (%)', 'unit': 'Unité'}


def esc(value) -> str:
    return html.escape('' if value is None else str(value), quote=True)


def field(name, label, value='', *, kind='text', extra=''):
    return f'<label for="{name}">{label}<input id="{name}" name="{name}" type="{kind}" value="{esc(value)}" {extra}></label>'


def render_preview(identifier, rotation=0):
    identifier = quote(identifier, safe='')
    return f'''<section id="preview" class="preview" aria-label="Scan du document">
      <div class="preview-toolbar"><span>Document original</span><div>
      <button type="button" class="quiet" hx-get="/preview/{identifier}?rotation={(rotation + 90) % 360}"
        hx-target="#preview" hx-swap="outerHTML" aria-label="Tourner le scan de 90 degrés">Tourner</button>
      <a href="/image/{identifier}?rotation={rotation}" target="_blank" rel="noopener">Agrandir</a></div></div>
      <div class="paper"><img src="/image/{identifier}?rotation={rotation}" alt="Scan du document {identifier}"></div>
      <p class="preview-caption">Compare les champs au scan. La rotation ne modifie pas le fichier source.</p>
    </section>'''


def proposal(prediction):
    if not prediction:
        return '<p>Aucune proposition disponible pour ce document.</p>'
    if prediction.get('status') != 'ok':
        return '<p>Gemma n’a pas produit de réponse exploitable. Annote uniquement les champs lisibles dans le scan.</p>'
    values = prediction.get('prediction', {})
    pairs = ''.join(f'<div><dt>{label}</dt><dd>{esc(values.get(key))}</dd></div>' for key, label in HEADERS.items())
    warnings = ''.join(f'<li>{esc(value)}</li>' for value in values.get('warnings', []))
    lines = ''.join('<li>' + ' · '.join(f'{label} : {esc(line.get(key))}' for key, label in LINE_FIELDS.items()) + '</li>'
                    for line in values.get('lines', []))
    return f'<dl class="proposal-fields">{pairs}</dl><ol>{lines}</ol><ul>{warnings}</ul><p>Une proposition ne devient jamais une référence automatiquement.</p>'


def render_page(rows, row, prediction, stats, version, image_sha, csrf, submitted=None,
                message='', reviewer='', fragment=False, queue_filter='all'):
    identifier = row['id']
    safe_id = quote(identifier, safe='')
    reference = row.get('reference') or {}
    values = {key: reference.get(key) or '' for key in HEADERS}
    lines = row.get('draft_lines') or reference.get('lines') or []
    form = {'reviewer': row.get('reviewed_by') or reviewer,
            'document_type': row.get('document_type') or 'unknown', 'notes': row.get('notes', ''),
            'line_count': str(len(lines)), 'line_items_complete': 'on' if row.get('line_items_complete') else '',
            'version': version, 'image_sha256': image_sha, **values}
    for index, line in enumerate(lines):
        form.update({f'line_{index}_{key}': line.get(key) or '' for key in LINE_FIELDS})
    if submitted is not None:
        form.update(submitted)
    try:
        line_count = min(100, max(0, int(form['line_count'])))
    except ValueError:
        line_count = 0
    hidden = ''.join(f'<input type="hidden" name="{key}" value="{esc(value)}">' for key, value in {
        'csrf': csrf, 'version': form['version'], 'image_sha256': form['image_sha256'], 'line_count': line_count}.items())
    options = ''.join(f'<option value="{key}" {"selected" if form["document_type"] == key else ""}>{label}</option>' for key, label in TYPES.items())
    header_fields = ''.join(field(key, label, form.get(key), kind='date' if key == 'issue_date' else 'text',
                                 extra='inputmode="decimal"' if key == 'total' else 'maxlength="200"') for key, label in HEADERS.items())
    line_fields = ''
    for index in range(line_count):
        inputs = ''.join(field(f'line_{index}_{key}', label, form.get(f'line_{index}_{key}'),
                               extra='inputmode="decimal"' if key in {'quantity', 'unit_price', 'tax_rate'} else '')
                         for key, label in LINE_FIELDS.items())
        line_fields += f'<fieldset class="line"><legend>Ligne {index + 1}</legend><div class="line-fields">{inputs}</div></fieldset>'
    status = row.get('review_status') if row.get('review_status') in LABELS else 'pending'
    reviewed = stats['confirmed_documents'] + stats.get('excluded_documents', 0)
    filter_options = ''.join(f'<option value="{value}" {"selected" if queue_filter == value else ""}>{label}</option>'
                             for value, label in {'all': 'Tous les documents', **LABELS}.items())
    queue = ''
    for candidate in rows:
        candidate_status = candidate.get('review_status', 'pending')
        if queue_filter != 'all' and candidate_status != queue_filter:
            continue
        active = ' aria-current="page"' if candidate['id'] == identifier else ''
        queue += f'<a class="queue-item" href="/?id={quote(candidate["id"], safe="")}&amp;filter={esc(queue_filter)}" {active}>'
        queue += f'<span>{esc(candidate["id"])}</span><small>{LABELS.get(candidate_status, LABELS["pending"])}</small></a>'
    position = next(index for index, candidate in enumerate(rows) if candidate['id'] == identifier)
    previous_id = rows[max(0, position - 1)]['id']
    next_id = rows[min(len(rows) - 1, position + 1)]['id']
    choices = ''.join(f'<option value="{esc(candidate["id"])}" {"selected" if candidate["id"] == identifier else ""}>{esc(candidate["id"])}</option>' for candidate in rows)
    notice = f'<div class="notice" role="status">{esc(message)}</div>' if message else ''
    if status == 'confirmed' and row.get('reviewed_image_sha256') != image_sha:
        notice += '<div class="notice" role="alert">Le scan a changé depuis la confirmation. Vérifie-le à nouveau avant d’exporter.</div>'
    checked = 'checked' if form.get('line_items_complete') == 'on' else ''
    export_disabled = 'aria-disabled="true" tabindex="-1" class="button disabled"' if not stats['confirmed_documents'] else 'class="button"'
    workspace = f'''<main id="workspace">
      <header class="masthead"><a href="/" class="brand"><span class="brand-mark"><img src="/assets/logo.png" alt=""></span>
      <span>PanlyackERP<small>Revue des références</small></span></a><span class="local-tag">Atelier local</span>
      <a href="/export" {export_disabled} download>Exporter les références</a></header>
      <section class="overview" aria-label="Avancement"><div>
      <h1>Revue des documents.</h1><p>Compare au scan, puis confirme ou écarte la pièce.</p></div>
      <div class="progress"><span><strong>{reviewed}</strong> / {len(rows)} documents revus</span>
      <progress value="{reviewed}" max="{len(rows)}" aria-label="Documents revus"></progress>
      <small>{stats['confirmed_documents']} confirmées · {stats.get('excluded_documents', 0)} écartés · {stats['pending_documents']} à revoir</small></div></section>
      <div class="workspace-grid"><aside class="queue"><h2>Documents</h2><form method="get" action="/" hx-get="/" hx-target="#workspace" hx-swap="outerHTML">
      <label for="queue-filter">Afficher</label><select id="queue-filter" name="filter">{filter_options}</select><button class="quiet" type="submit">Filtrer</button></form>
      <nav aria-label="Liste des documents" hx-boost="true" hx-target="#workspace" hx-swap="outerHTML">{queue or '<p>Aucun document dans ce filtre.</p>'}</nav></aside>
      <div class="review-area"><div class="document-nav"><div><span class="badge {status}">{LABELS[status]}</span><strong>{esc(identifier)}</strong><span>{position + 1} / {len(rows)}</span></div>
      <nav aria-label="Changer de document" hx-boost="true" hx-target="#workspace" hx-swap="outerHTML"><a href="/?id={quote(previous_id, safe='')}">Précédent</a><a href="/?id={quote(next_id, safe='')}">Suivant</a></nav></div>
      <form class="mobile-selector" method="get" action="/"><label for="document-choice">Aller au document</label><select id="document-choice" name="id">{choices}</select><button type="submit" class="quiet">Ouvrir</button></form>
      {notice}<div class="review-columns">{render_preview(identifier)}
      <section class="reference-panel" aria-labelledby="reference-title"><div class="panel-heading"><h2 id="reference-title">Référence à confirmer</h2>
      <p>Un champ illisible reste vide. Les montants utilisent un point : 12.50.</p></div>
      <form id="reference-form" method="post" action="/review/{safe_id}" hx-post="/review/{safe_id}" hx-target="#workspace" hx-swap="outerHTML" hx-disabled-elt="find button" hx-indicator="#saving">
      {hidden}<div class="identity-fields">{field('reviewer', 'Ton nom', form['reviewer'], extra='maxlength="80" autocomplete="name"')}
      <label for="document_type">Type de document<select id="document_type" name="document_type">{options}</select></label></div>
      <fieldset><legend>Champs lisibles de la facture</legend><div class="header-fields">{header_fields}</div></fieldset>
      <fieldset><legend>Lignes du document</legend><p class="hint">Laisse les quantités, unités ou taxes inconnues vides.</p>{line_fields}
      <button type="submit" class="quiet" name="action" value="add_line" formnovalidate>+ Ajouter une ligne</button>
      <label class="check"><input type="checkbox" name="line_items_complete" {checked}>Toutes les lignes ont été relevées</label>
      <p class="hint">Sinon, les lignes restent un brouillon et ne sont pas comptées comme une liste complète.</p></fieldset>
      <label for="notes">Notes ou motif d’exclusion<textarea id="notes" name="notes" rows="3" maxlength="2000">{esc(form['notes'])}</textarea></label>
      <div class="actions"><button type="submit" name="action" value="confirm" class="primary">Confirmer la référence</button>
      <button type="submit" name="action" value="draft" class="quiet" formnovalidate>Enregistrer le brouillon</button>
      <button type="submit" name="action" value="exclude" class="danger" formnovalidate>Écarter du lot de factures</button></div>
      <span id="saving" class="htmx-indicator" role="status">Enregistrement…</span>
      <p class="hint">Confirmer attribue la revue à ton nom. Cela ne lance aucun entraînement ni opération comptable.</p></form>
      <details class="model-proposal"><summary>Voir la proposition de Gemma</summary>{proposal(prediction)}</details>
      <details class="source"><summary>Source et traçabilité</summary><p>{esc(row.get('source_file'))}</p><p>Référence originale : {esc(row.get('source_sha256'))}</p>
      <p>{esc(row.get('reviewed_by'))} {esc(row.get('reviewed_at'))}</p></details></section></div></div></div>
      <footer>Les exports conservent les champs inconnus. Les droits des sources restent applicables. Aucun entraînement automatique.</footer></main>'''
    if fragment:
        return workspace
    return '''<!doctype html><html lang="fr"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
      <meta name="htmx-config" content='{"allowEval":false,"allowScriptTags":false,"includeIndicatorStyles":false,"responseHandling":[{"code":"204","swap":false},{"code":"[23]..","swap":true},{"code":"409","swap":true},{"code":"422","swap":true},{"code":"[45]..","swap":false,"error":true}]}'>
      <title>Revue des références · PanlyackERP</title><link rel="icon" href="/assets/logo.png"><link rel="stylesheet" href="/assets/review.css">
      <script src="/assets/htmx.min.js" defer></script><script src="/assets/review.js" defer></script></head><body><a class="skip-link" href="#reference-title">Aller aux champs</a>''' + workspace + '</body></html>'
