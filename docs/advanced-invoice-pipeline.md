# Advanced invoice extraction and US account-role pilot

## Scope

The existing confirmed-invoice posting workflow remains authoritative: exact quantities and money, organization-scoped stock, balanced account roles, explicit payment evidence, transactional/idempotent posting and audit. Extraction never authorizes posting. A supplier invoice is not evidence of payment by itself. Operating expenses do not create stock.

This slice improves OCR table evidence, product-row instructions, arithmetic diagnostics and one bounded corrective vision pass. It adds a lightweight US account-role LoRA with local retrieval. It does not claim certified US GAAP compliance, infer tax jurisdiction, generate missing payments or replace human review. Account codes are organization-specific.

## Deliverables and acceptance

- Preserve middle-of-document OCR rows, provide automatically grouped table observations and bounded detail views without source-label boxes.
- Request product names, quantities, unit prices, units and tax rates; reject impossible schema values and flag exact-decimal total/payment inconsistencies. Retry once using diagnostic messages without silently changing amounts.
- Preserve confirmed invoice → stock/accounting/cash behavior and prove representative quantity/partial-payment/expense cases using isolated database tests.
- Generate reproducible bounded US perpetual-inventory examples using the existing rules engine, with explicit purpose, supported tax treatment and settlement facts; separate training/validation products and avoid the 108 reserved invoice test images.
- Train a small rank-eight text LoRA with the cached model, record provenance, memory, losses and generated-role validation. No invoice images or personal payment records are misrepresented as reviewed accounting labels.
- Serve model suggestions only when an explicitly configured local adapter exists. Validate allowed roles and compare them with deterministic expected roles; discrepancies require review and never post data.
- Document actual checks, measured extraction/training results and remaining data/quality limitations.

## Data and tax boundaries

The Beancount example declares itself generated. Iowa's public-sector ledger does not supply balanced corporate journal labels. Neither is used as authoritative double-entry supervision. Purchases versus expenses must be chosen by the operator from business purpose, not guessed from product names. Larger, accountant-reviewed examples and real supplier/layout holdouts remain necessary before claiming accounting or extraction accuracy.

## Bounded US posting policy

New registrations can select a US demo chart. Existing organizations and immutable entries are not migrated. US purchases treat entered purchase tax as nonrecoverable and include it in inventory/expense cost. Stock receipt unit costs and the reporting projection use the same gross cost, with exact decimals and a precision guard. Sales credit buyer-imposed tax collected for government to OUTPUT_TAX separately from revenue. Confirmation displays these assumptions. The operator must establish the applicable jurisdiction, tax treatment and rate; the model never selects them.

This is a deliberately limited policy: seller-imposed sales taxes, use-tax accruals, exemptions/resale-certificate verification, income tax, filing and nexus determination are not implemented. [IRS Publication 334](https://www.irs.gov/publications/p334) distinguishes buyer- and seller-imposed sales taxes; [CDTFA guidance](https://www.cdtfa.ca.gov/taxes-and-fees/applying-tax-sales-purchases-faq.htm) illustrates jurisdiction-specific sales/use-tax rules. These tax references do not certify GAAP compliance.

## Local retrieval and training

`app/knowledge/us-accounting.json` contains six short, sourced reference cards. `build-accounting-context.py` extracts 40 generated Beancount ledger snippets and 40 unique Iowa expenditure-description snippets, with original-file SHA-256 and provenance. Iowa payees, amounts and identifiers are omitted. This is a transformed excerpt of State of Iowa / State Accounting Enterprise data under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/); its [publisher page](https://data.iowa.gov/catalog/dataset/1074) remains authoritative for the dataset, not for corporate journal labels.

Retrieval uses explicit operation/tax filters, structural example matching and a small lexical match for public vocabulary. No embedding service, vector database, ERP ledger access or cross-organization private corpus is introduced. At most four references enter the prompt; JSON corpora are capped at 4 MB. Products and retrieved text are untrusted observations. Read-only proposals return their sources and remain independently checked against Go accounting roles. A mismatch never authorizes posting.

The v2 generator produces 276 synthetic examples: 184 training, 46 validation and 46 reserved tests. It focuses on CASH/BANK, purchases, operating expenses, sales, later supplier/customer settlements and both directions of own cash–bank transfers, with explicit supported tax treatment. Transfer examples are balanced educational cases; they do not add a bank-transfer execution feature. Training uses rank-eight LoRA on cached SmolVLM-256M, target-only loss and the training partition as RAG examples. Validation/test products are separate, but accounting structures repeat; this measures interpolation to new product descriptions, not independent accounting expertise. The 108 reserved invoice test images remain untouched.

The earlier no-tax model scored 25/36 exact generated-role matches after six epochs (8/36 for the initial three-epoch run), and is preserved separately. Its prompt is incompatible with v2 and the runtime rejects old adapters.

The v2 RAG+LoRA run completed four epochs: 46/46 schema-valid and exact role matches on synthetic validation, best target validation loss 0.0009238644 (baseline 0.9383456), peak allocated CUDA memory 2.654 GiB. The 46 final test examples were not evaluated. The dataset SHA-256 is `2acf56dbdf5d8034ebe08b67a9b95070a897e5ac52b222eb68d6b5aa8e06ca50`; the runtime verifies both example and public-context hashes against the training report. Because retrieval supplies matching structural training examples, this result must not be described as independent accounting accuracy or evidence that fine-tuning alone caused the improvement.

```powershell
go run ./cmd/accounting-examples --output D:/PanlyackERP-training/us-account-role-v2-examples.json
D:/PanlyackERP-training/runtime/Scripts/python scripts/build-accounting-context.py --output D:/PanlyackERP-training/us-public-context-v2.json
$env:ACCOUNTING_PUBLIC_CONTEXT_PATH = 'D:/PanlyackERP-training/us-public-context-v2.json'
D:/PanlyackERP-training/runtime/Scripts/python scripts/train-us-accounting.py --dataset D:/PanlyackERP-training/us-account-role-v2-examples.json --output D:/PanlyackERP-training/us-account-role-rag-v2 --epochs 4
```

Output paths must be new. Full reports, adapter weights and local data remain outside Git. Sources never become human-confirmed labels merely by appearing in retrieval.

## Runtime and measured extraction limitations

Use `AI_PYTHON` to select the optional inference environment; `ACCOUNTING_ADAPTER_PATH`, `ACCOUNTING_RAG_EXAMPLES_PATH` and `ACCOUNTING_PUBLIC_CONTEXT_PATH` configure the local advisor. Enable `EXTRACTOR_MODE=ollama`, `OCR_ENABLED=1`, `OCR_GPU=1` and `VISION_DETAIL_ENABLED=1` for the RTX 4060 laptop. Runtime packages include the project's pinned AI dependencies (`pip install -e './ai-service[ocr,accounting]'`), CUDA-enabled PyTorch, Transformers 4.57.1 and PEFT 0.17.1. Hugging Face loading is local-only. Configure HF_HOME for a different cache location.

OCR uses five automatically positioned views, bounded observations, normalized boxes, middle-row preservation and row grouping. Gemma receives original pages plus one explicitly overlapping detail view, schema-constrained generation, product/unit/payment guidance and an exact-arithmetic check. One corrective reading is permitted within the request budget. Unsupported tax/discount/rounding cases remain visible for correction; amounts are never fabricated to balance a document.

A first CPU/Ollama smoke failed because the 6,470-token prompt exceeded the model's default 4,096-token context. The configured context was increased. A subsequent GPU OCR smoke returned schema-valid data in 39.6 seconds and found all four product rows on one previously used synthetic validation invoice. It still confused the customer with the supplier, produced incorrect units/currency and differed from the printed total by 0.002 due to four-decimal line-tax arithmetic versus printed cent rounding. The draft was flagged with zero global confidence. This is a diagnostic smoke, not an accuracy score; follow-up prompts explicitly address partner role, printed currency and unit fabrication.

Local model/vision calls are serialized to limit memory pressure. Slots stay occupied until actual work finishes even if the HTTP request times out. Account-role inference runs on CPU in a separate bounded process, returning model/import memory to the OS after each proposal; Windows timeout cleanup stops the worker process tree. An earlier attempt to run inference, builds and tests concurrently exhausted workstation memory. Do not run training and the full inference stack simultaneously on this 16 GB RAM / 8 GB VRAM machine. Gemma itself is not fine-tuned by this slice; the existing experimental image-header LoRA is preserved and is not misrepresented as a product-line model.

## Verification completed

- `go test ./...`, `go vet ./...`, template generation and the page-currency/US-policy regression test passed.
- `go test ./internal/workflows/registration ./internal/workflows/invoiceposting -count=1 -v` passed with TEST_DATABASE_URL and actual PostgreSQL. Tests cover no-tax and taxed US purchases/expenses/sales, tax-inclusive inventory value, sales-tax liability, later settlements, organization isolation, atomic failure and repeat posting.
- `.venv/Scripts/python -m pytest ai-service/tests -q`: 67 passed; one pre-existing Starlette/httpx deprecation warning remains.
- A fresh isolated HTTP QA organization exercised real OCR/Gemma upload (47.6 s), manual confirmation of clearly synthetic accounting edits, real CPU LoRA+RAG proposal (41.03 s), US account resolution and idempotent purchase/sale posting. Its final exact balances were stock 27 USD, cash/bank aggregate 4,990 USD, AP 34 USD, AR 33.20 USD, revenue 40 USD and COGS 27 USD. The source image was not represented as a real US transaction.
- The earlier full-flow scan found all four products and corrected currency/units, but still selected the customer as supplier. Recipient-block OCR diagnostics now refocus the single corrective pass on the letterhead. Only an OCR-supported supplier name can be merged from this focused view; product amounts, quantities, taxes and currency remain from the original full-page reading. The final real smoke returned HTTP 200 in 25.48 seconds with AFRICA OFFICE & TECH, EUR, all four product names/quantities/unit prices and 20% tax. The 0.002 printed-cent rounding difference remains flagged and requires an explicit rounding-policy enhancement, not an invented amount adjustment. This is still one repeatedly inspected synthetic validation image, not independent accuracy evidence.
- The real registration page was inspected in the in-app browser and exposes the US/FR chart choice.
- A final real isolated-worker call handled a taxed, partially bank-paid sale including COGS: HTTP 200, policy match, 40.41 seconds; no posting was requested or performed by the AI endpoint.

Ignored local artifacts: `var/us-rag-flow-report.json`, `var/final-ocr-gemma-draft.json` and `var/us-rag-proposal.html`. These are smoke evidence, not a labelled real-world evaluation set.
