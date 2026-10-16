# Local annotation review

The workbench lets a human classify source scans and record extraction references. It is separate from ERP invoice approval: it has no database connection and never posts invoices or starts training. It uses FastAPI, server-rendered HTML and the existing local HTMX asset.

## Start

From the repository root:

```powershell
./scripts/review-invoices.ps1
```

Open http://127.0.0.1:8093. Keep the terminal running; Ctrl+C stops the workbench. Defaults select `var/evaluations/invoices-100` and `runs/date-guidance/predictions.json`. Override `-Dataset`, `-Predictions` or `-Port` for another local run. No download or model invocation occurs.

The service binds to loopback. It is a single-process, single-operator local tool. The reviewer name provides attribution, not authenticated identity. Do not expose it through a tunnel or shared server. CSRF tokens, same-origin POST checks, a host allowlist, bounded forms and escaped HTML protect this local workflow; they do not replace production authentication.

## Review

1. Read the scan and classify its actual type. The archival invoice class also contains checks and purchase requests. Rotation changes only the display.
2. Enter legible fields independently. The collapsed Gemma proposal is read-only and never copied automatically. Existing assistant drafts remain unconfirmed until a human verifies them.
3. Keep unknown fields blank. Use unambiguous ISO calendar dates and decimal strings with a point. Preserve printed currencies, including historical codes, without conversion.
4. Add line items as needed. Until all lines are marked complete, they remain draft lines and are not exported or scored as a complete list. Clear every field in a line to remove it on save.
5. Save a draft, confirm the reference under your name, or exclude a non-invoice with an explicit type and reason. Confirmation requires at least one known field; it does not imply all fields are known.

Saving atomically updates `annotations.json` and appends the previous state to `review_history`. File-version and image hashes reject stale edits. Reload after a conflict before editing again. Saving a draft revokes previous confirmation. Embedded history is not an external backup. Multiple service processes and concurrent external editors are unsupported.

## Export and evaluate

The ZIP contains `reviewed-pairs.jsonl`, PNGs and a manifest of skipped documents. Only named, timestamped, confirmed invoice references with an unchanged reviewed image are exported. Unknown fields stay null. Draft lines and unconfirmed or excluded documents are omitted. Source hashes and reviewer attribution accompany each pair.

Pairs carry `target_scope: partial_reference`. This is **not** a complete Invoice-schema SFT corpus: missing fields, payments, SKU mapping and full line annotations still need a task-specific specification. Source usage rights apply. Exporting never launches training.

Recompute scores after review:

```powershell
Set-Location ai-service
../.venv/Scripts/python.exe -m app.evaluation score --dataset ../var/evaluations/invoices-100 --predictions ../var/evaluations/invoices-100/runs/date-guidance/predictions.json
```

Scoring requires `document_type: invoice`; older confirmed annotations without a type must be classified explicitly. Never train on this development set and present its score as a held-out evaluation. See [evaluation limitations](invoice-evaluation.md). Corpora, exports and browser fixtures stay under ignored `var/`.
