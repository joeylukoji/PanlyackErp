# Local and public invoice review pack

The user requested 50 local documents plus 50 public documents, with supplied CSV/JSON labels and an explanation of available annotation fields. The resulting local artifact is `C:/Users/Joy/Downloads/PanlyackERP_training_review_100`. No source download was resumed and no model training or ERP import ran.

## Contents and provenance

- `local_50`: 50 selected synthetic receipts from the 58 image/label pairs under `C:/Users/Joy/Downloads/facture_datasets/outputs/photos_factures`. The four root-level examples and their older label files are not used. Sampling uses seed 42 across eight supplied categories. Each selected image is copied byte-for-byte; the complete original 58-row CSV and JSON are retained separately, unchanged.
- `public_50`: 50 real archival RVL-CDIP invoice-class scans from the already downloaded first shard. Original TIFFs are retained; canonical PNGs are EXIF-oriented RGB renders. Source class encoding is taken from the mirror metadata, not the original RVL category ordering. The 50 source image byte hashes and labels were checked against original Parquet rows; the shard hash matches pinned publisher Git LFS metadata.
- Both lots contain per-document label/provenance JSON, `annotations.json`, `review.csv`, an empty predictions file and a static visual index. Every record is pending, `document_type` is unknown, and the human reference is null. The public dataset provides classification only: no invoice-field or line-item labels are invented.
- `public_datasets`: copied annotation/documentation/license snapshots for the already downloaded InvoiceOCR-Synth labels (938 fictional records), Iowa public-payment sample (10,000 rows plus column metadata), and fictional Beancount ledger examples. These supporting files do not add invoice images or matched invoice-to-journal targets to the 100-document pack. Their different licenses and limitations remain separate.

Local source fields are normalized into an inspection draft with ISO dates and decimal strings. Fractional tax rates and percent rates are distinguished; discounts, tendered amounts and change are preserved. Missing SKU, unit, accounting account and per-line tax rates are not inferred. Original numeric JSON remains unchanged in the source snapshot; parse it using Decimal for arithmetic.

The public selection excludes previously used local evaluation image hashes (100 unique hashes on this run). This prevents exact-byte reuse of those images; supplier/layout independence is not established. There is no train/validation/test split and the pack is not ready-to-run SFT input. Human screening and source usage rights remain required.

## Reproduce

From the repository root, with Pillow and, for original-row verification, PyArrow available in the selected Python interpreter:

```powershell
$env:PYTHONPATH = Join-Path (Get-Location) 'ai-service'
python -m app.training_review --local C:/Users/Joy/Downloads/facture_datasets --public var/datasets/real-invoices-rvl-cdip --output C:/Users/Joy/Downloads/PanlyackERP_training_review_100_NEW --exclude-evaluations var/evaluations --verify-parquet
```

The utility refuses to overwrite an existing pack. It never executes the supplied generator, downloads files or approves references. PyArrow is used only by the optional preparation verification step; it is not added to the production AI-service dependencies. The completed run used the existing system Python installation with Pillow/PyArrow.

Open `index.html` directly, or serve only the package directory:

```powershell
python -m http.server 8094 --bind 127.0.0.1 --directory C:/Users/Joy/Downloads/PanlyackERP_training_review_100
```

The root guide explains manual review and opening a lot with the existing annotation workbench. CSV changes do not synchronize automatically to `annotations.json`.

## Verification actually completed

- All 100 canonical images decoded; original and canonical hashes checked; 100 distinct original hashes.
- All 50 public selections matched their original Parquet row labels and embedded image bytes. Verified shard SHA-256: `f8eee0665cb93f2f2227b250865038f1ca2e9904e36b8804ec043a6b8d656627`.
- No arithmetic or source CSV/JSON discrepancies found in the selected 50 local label rows. This does not establish agreement with the visible image content.
- 288 package file hashes verified against the generated checksum list; 200 local HTTP image/label links returned success. Desktop visual indexes for both lots were rendered in Chrome and inspected. These checks are not human reference confirmation.
- Python suite: 31 tests passed, with the existing upstream Starlette/httpx deprecation warning. New tests cover exact-decimal normalization, preserved raw source labels, separate pending lots, duplicate/insufficient-source handling, evaluation hash exclusion, path containment and discrepancy reporting without silent correction.

Artifacts, images, ledgers and local review sessions stay outside Git. Only the preparation utility, tests and technical documentation are committed.
