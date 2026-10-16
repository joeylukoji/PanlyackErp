# Local invoice extraction evaluation

The evaluation uses the same `OllamaExtractor`, request validation and invoice schema as the FastAPI service, without starting the ERP or writing to its database. It is a baseline measurement, not model training. All requests go to the explicitly configured model URL (localhost by default). No hosted model provider is used.

## Prepared local dataset

`var/evaluations/invoices-50/` contains 50 PNG scans selected from the already downloaded RVL-CDIP invoice images. Source byte duplicates are excluded. Selection is deterministic (seed 42), distributed across orientation and file-size buckets as a simple layout-diversity proxy. It is not supplier-stratified or a claim of statistical representativeness. `selection.json` records the procedure and source directory; `annotations.json` retains original filenames and SHA-256 hashes. The source archives remain unchanged, and their stopped download is not restarted.

Open `var/evaluations/invoices-50/index.html` to inspect the scans, then open individual images at full resolution. Source reuse conditions still apply; see `var/datasets/real-invoices-rvl-cdip/README.md`. Data and model results stay under Git-ignored `var/`; the reusable runner and its tests are versioned.

Source classification is not ground truth: visual inspection of the first selected scan found a check/payment voucher rather than a standard invoice. Human review must screen document type as well as extracted fields. Such documents can expose forced invoice extraction and must not silently become training examples. Unknown dates and missing line items also conflict with the production schema's required fields; a validation failure is retained as a real baseline limitation.

## Run from PowerShell

From the repository root:

```powershell
Set-Location ai-service
../.venv/Scripts/python.exe -m app.evaluation prepare --source ../var/datasets/real-invoices-rvl-cdip/images --output ../var/evaluations/invoices-50 --count 50
../.venv/Scripts/python.exe -m app.evaluation run --dataset ../var/evaluations/invoices-50 --output ../var/evaluations/invoices-50/runs/baseline --model gemma3:4b
```

Preparation refuses to overwrite an existing dataset, protecting manual annotations. Each run requires a new output directory. Use `--limit 1` for a smoke test and a different output directory for subsequent runs. The production adapter's 90-second HTTP timeout also applies to this benchmark. Loading a cold model may exceed it; record that failure rather than silently increasing production timeouts or switching to demo extraction.

Results are saved after every document to `predictions.json` and `report.json` inside the run directory. Failures are recorded and the run continues. A schema-valid response is not evidence that the content is correct. Missing and failed predictions count as mismatches when a reviewed reference exists. Model responses retain all warnings and confidence values, which are not calibrated probabilities.

## Human reference annotations

No document starts with a confirmed reference. Model output is stored separately and never copied into ground truth or approved automatically. For each document in `annotations.json`:

1. Read the source scan independently of the prediction where possible.
2. Replace `reference: null` with the fields visible in the image, following `reference-template.json`. Unknown or illegible fields remain `null`; do not invent dates, payment evidence, SKUs, taxes or missing prices.
3. Store monetary values and quantities as decimal strings. Dates use `YYYY-MM-DD` when unambiguous. Preserve document line order; annotate every line if scoring line count. Omit `lines` or set it to `null` when a complete line list cannot be established.
4. Only after a human review, set `review_status` to `confirmed`, fill `reviewed_by` and record an ISO timestamp in `reviewed_at`. Use `notes` for ambiguity or non-invoice documents.

Example of the reference object (illustrative only, never a real annotation):

```json
{"number": "EXAMPLE-1", "partner": "Example supplier", "issue_date": null, "currency": "USD", "total": "12.50", "lines": null}
```

Calculate metrics after reviewing references:

```powershell
../.venv/Scripts/python.exe -m app.evaluation score --dataset ../var/evaluations/invoices-50 --predictions ../var/evaluations/invoices-50/runs/baseline/predictions.json
```

Only confirmed references with a named reviewer and non-null fields are scored. Financial comparisons use `Decimal`, text comparisons normalize whitespace and case, and line items are compared in document order. The report shows scored-field coverage and per-field matches; `field_accuracy` remains `null` until reference fields have actually been reviewed. JSON accuracy ratios and elapsed seconds are measurements, not financial values.

## Decision after the baseline

The local baseline completed on 2026-10-16 using installed `gemma3:4b` (digest `68ceab7e468a31c988b44d5a4984a0e21f58c4f7158c6ce29c1804babac38df7`): 50 attempted documents, 4 schema-valid responses and 46 Pydantic validation failures on `issue_date`. The initial cold smoke test timed out at 90 seconds; the warm smoke test returned an invalid date format. These are actual local adapter results, not a human extraction-accuracy score. All 50 references remain pending human confirmation. The local report is `var/evaluations/invoices-50/runs/baseline/report.json`, with per-document outcomes in `predictions.json`.

The first remedial experiment should improve explicit date-format guidance and test the known failures without fabricating dates, alongside screening non-invoice source documents. A schema-pass improvement alone would still need independent field-level review. No prompt change or model fine-tuning was applied during this baseline.

Inspect supplier, number, date, currency, total and line-item errors separately. The current ERP schema has line tax rates but no separate header subtotal or tax-total field; this benchmark does not pretend to measure fields absent from the adapter contract. A separate held-out set, grouped by supplier/layout and checked for duplicates, is required before claiming generalization or comparing a fine-tuned model. This 50-document pilot must not be reused as training data and then presented as an independent test.

Fine-tuning requires reviewed image/reference pairs and suitable source rights. The real Iowa payment sample does not provide these pairs or balanced debit/credit journal targets. It is not imported into this extraction benchmark. Human confirmation and deterministic accounting checks remain mandatory in the ERP.

## Expanded date-guidance experiment

The follow-up uses 100 documents from the same downloaded source images, without restarting the archive download. Prepare and run it separately so neither the original 50-image dataset nor its baseline predictions are overwritten:

```powershell
../.venv/Scripts/python.exe -m app.evaluation prepare --source ../var/datasets/real-invoices-rvl-cdip/images --output ../var/evaluations/invoices-100 --count 100
../.venv/Scripts/python.exe -m app.evaluation run --dataset ../var/evaluations/invoices-100 --output ../var/evaluations/invoices-100/runs/date-guidance --model gemma3:4b
```

The first 50 source hashes were checked against the original pilot and match in order; the remaining 50 are additional documents. This allows a same-document comparison of schema failures before/after prompt guidance. The prompt now explicitly requests ISO dates, distinguishes invoice dates from due/payment dates, and asks for an empty date with a warning instead of fabricating missing or ambiguous information. Empty dates still fail the required-field schema safely. A calendar validator also rejects impossible dates such as 2026-02-29; string formatting alone previously accepted them.

This is a development experiment, not an independently held-out accuracy evaluation: the earlier 50 errors informed the prompt revision. No fine-tuning or model-weight changes are involved. New predictions remain unconfirmed and do not update reference annotations.

Visual checks reinforce the limits: document 001 is a check/payment voucher, and document 005 is a shipping memorandum/warehouse receipt. On document 005, Gemma returned a schema-valid date of 2022-12-07 that does not match the handwritten date visible in the scan. Those observations are assistant review notes, not human-confirmed labels. More documents and more ISO-formatted outputs do not establish correct dates or correct document types; human screening remains necessary.

Assistant review notes were added for the first nine scans inspected. Document 003 contains an unconfirmed visual draft of a few reference fields, including its printed March 19, 1988 invoice date; handwritten later dates are payment/reminder annotations. Its status remains `pending`, with no human reviewer, and it is excluded from accuracy scoring until confirmed. Several other inspected pages are vouchers, a revised budget, an internal approval sheet or a ticket/receipt. This corpus must therefore be screened rather than described as 100 confirmed invoices.

Additional visual notes cover documents 051, 060 and 065 from the new half. They include a French/Dutch invoice with several date markings, a German pro-forma invoice denominated in DM, and a purchase requisition. The archive is not exclusively US supplier invoices. Historical currencies such as DEM are outside the ERP's current USD/EUR/CDF/ZAR enum; schema-valid currency output can therefore be incorrect. These examples require source screening and human references, not automatic relabeling to a supported currency.

The expanded run completed on 2026-10-16: 100 attempts, 96 schema-valid responses and 4 rejected responses. The failures were an empty date (017), invalid/truncated JSON (024 and 073), and an impossible month in `1992-13-07` (084), rejected by the new calendar validator. On the same first 50 images, accepted responses increased from 4 to 48; the added 50 produced 48 accepted responses. This is schema conformity only, not 96% extraction/date accuracy. All 100 documents remain unconfirmed for scoring. Results and the same-document comparison are saved under `var/evaluations/invoices-100/runs/date-guidance/`.
