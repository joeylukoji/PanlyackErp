# Reproduction and implementation record

Author and maintainer: Joy. This record accompanies the three-page [technical article](article.md) and distinguishes implementation evidence from intended capabilities.

## How the application was built

1. **Deterministic foundation.** The initial vertical slice implemented exact monetary values, an invoice state machine, FIFO/AVCO valuation and balanced journal strategies. Module-owned PostgreSQL adapters and consumer-owned workflow interfaces coordinate a single posting transaction without an ORM. See [ADR 0001](../adr/0001-mvp-boundaries.md), `internal/workflows/invoiceposting/service.go`, `internal/modules/accounting/domain/journal.go` and `internal/modules/inventory/domain/valuation.go`.
2. **Document ingestion and review.** Bounded local uploads, SHA-256 provenance, explicit simulated extraction and a real Ollama adapter were added. Pydantic validates the extraction contract; Go independently validates exact totals and business transitions. Human confirmation is required. `ai-service/app/extractors.py`, `invoice_checks.py` and `schemas.py` contain this boundary.
3. **Connected screens and identity.** Server-rendered templ/HTMX views expose dashboard, invoice review, stock, accounting and cash. Owner login and isolated self-service registration provision sessions, charts and synthetic opening balances. Later partial settlements and account-filtered ledger views extend the same transaction rules. See [registration ADR](../adr/0003-local-test-registration.md).
4. **Separate authored and external documents.** App-created sales store structured JSON and render printable invoices without extraction. Supplier purchases and operating expenses retain file/camera ingestion. Expense posting skips stock. See [sales and expenses](../sales-and-expenses.md).
5. **Data preparation and lightweight experiments.** Review packs preserve source labels, CSV/JSON provenance, image hashes and parent-linked variants. The image-only header pilot preceded the OCR-assisted pilot. Training uses supplied synthetic references; no global invoice accuracy is established. See [expanded splits](../expanded-invoice-tests.md) and [OCR experiment](../ocr-and-augmentation.md).
6. **Runtime OCR and US suggestions.** Five automatic OCR views, table evidence, bounded vision correction and role diagnostics improve the review pipeline. A separate US account-role adapter provides retrieved, sourced suggestions; deterministic accounting remains authoritative. See [advanced pipeline](../advanced-invoice-pipeline.md).
7. **Presentation and localization.** Responsive landing/authentication/dashboard screens, a collapsible sidebar and persistent French/English selection complete the presentation. Displayed USD pricing is $49/200 monthly invoices, $79/500 and $150/unlimited. Billing and quota enforcement are not implemented.

The existing Git history records these increments. It is preserved on publication, not replaced by an artificial first-commit history. The architecture graph is an earlier navigation snapshot, not a current completeness check.

## Runtime and model inventory

| Component | Purpose | Adaptation and runtime status |
| --- | --- | --- |
| EasyOCR 1.7.2 | French/English detection and recognition; spatial/table observations | Pretrained CRAFT/Latin recognition; no project-specific OCR-weight training; optional live runtime |
| Gemma 3 4B through Ollama | Invoice headers and product rows from bounded images plus OCR | No project fine-tuning; configured live recognition path |
| SmolVLM-256M-Instruct, image-only LoRA | Four header fields | Three-epoch pilot, experimental, not loaded by the ERP |
| SmolVLM-256M-Instruct, image + OCR LoRA | The same four header fields | Rank 8, three epochs; experimental, not loaded by the ERP |
| SmolVLM-256M-Instruct, US roles + retrieval LoRA v2 | Read-only debit/credit role sets | Rank 8, four epochs; optional connected advisor, rules checked, not production-certified |

The accounting model returns roles only, never journal amounts or posting authority. Product descriptions and retrieved snippets are untrusted observations. Local inference is bounded and serialized; the accounting worker runs on CPU in a separate process and releases memory on exit. Connected models are not all resident simultaneously.

## Local setup

Use the root [README](../../README.md) for prerequisites, `.env.example`, PostgreSQL and local startup. Set secrets locally; do not commit `.env`.

```dotenv
EXTRACTOR_MODE=ollama
MODEL_NAME=gemma3:4b
OCR_ENABLED=1
OCR_GPU=1
VISION_DETAIL_ENABLED=1
ACCOUNTING_ADAPTER_PATH=<local-us-role-adapter-directory>
ACCOUNTING_RAG_EXAMPLES_PATH=<local-generated-accounting-examples.json>
ACCOUNTING_PUBLIC_CONTEXT_PATH=<local-public-context.json>
```

Install the optional OCR/accounting dependencies from `ai-service/pyproject.toml` in a separate environment. Install CUDA-compatible PyTorch and the EasyOCR recognition files separately. Runtime model downloads are disabled. Select the environment with `AI_PYTHON`; use available loopback ports if Windows reserves defaults. `scripts/start-local.ps1 -Check` validates startup configuration.

The default extractor is explicitly simulated. Real recognition requires separately obtained model files; cloning the repository does not download checkpoints, train adapters or reproduce measured quality automatically.

## Data, adaptation and reproduction

The frozen base for the two SmolVLM families is `HuggingFaceTB/SmolVLM-256M-Instruct`, revision `7e3e67edbbed1bf9888184d9df282b700a323964`, with weights SHA-256 `74dea5904032e5ae99a2e0eef5179e6ac0f1dedc3ab0c7c2a5d4d387c843203e`. The model uses its publisher's Apache-2.0 license; Gemma weights have separate publisher terms and are not covered by this repository's license. EasyOCR's source distribution is Apache-2.0. No weights are redistributed here.

The expanded invoice pack comprises 193 distinct synthetic source documents and 65 mild variants: 142 training images (77 parents + 65 variants), 8 validation images and 108 reserved test images. Labelled crops remain with their parent partition and are not fed to runtime OCR as target boxes. Identity/hash checks reject overlap; shared generator layouts mean supplier/layout independence is not established. Source labels are not human-confirmed, and some validation supplier headings are not fully visible.

```powershell
# Requires the supplied local source corpus; paths are examples, not bundled data.
python scripts/prepare-ocr-dataset.py --help
python scripts/extend-ocr-dataset.py --help
python scripts/train-light-invoices.py --dataset <expanded-pack> --output <new-run-directory>
python scripts/evaluate-light-invoices.py --run <completed-run> --dataset <expanded-pack> --split validation

# Generate corporate examples from the bounded deterministic rules.
go run ./cmd/accounting-examples --output <new-examples.json>
python scripts/build-accounting-context.py --root <local-public-source-folder> --output <new-public-context.json>
# Set ACCOUNTING_PUBLIC_CONTEXT_PATH before training to include the same context.
python scripts/train-us-accounting.py --dataset <new-examples.json> --output <new-accounting-run> --epochs 4
```

Use a fresh output directory to retain prior runs. The accounting generator creates 276 cases (184/46/46); the retrieval examples use only the training partition. Repeated accounting structures make validation an interpolation test, not independent expertise. Neither the 108 invoice tests nor 46 accounting tests were evaluated for the reported checkpoints.

Public context contains 40 generated Beancount ledger snippets and 40 Iowa expenditure-vocabulary snippets, plus six sourced cards. Original source checksums and attributions are retained locally. Iowa payees, amounts and identifiers are omitted; transformed descriptions are attributed to State of Iowa / State Accounting Enterprise, [publisher dataset 1074](https://data.iowa.gov/catalog/dataset/1074), under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Beancount examples are explicitly generated and are not real transaction labels. These materials provide context, not accountant-reviewed corporate journal supervision. Public snapshots and source invoice collections are not published here.

## Evidence and verification

- [Invoice training report](evidence/invoice-ocr-training.json): original report fields plus source-report checksum.
- [Header generation summary](evidence/invoice-generation-summary.json): aggregate metrics only; no source images or label/prediction records.
- [Accounting training report](evidence/us-account-role-training.json): original report fields plus source-report checksum.
- [Current publication checks](evidence/publication-checks.json): commands actually run for this publication. A skipped PostgreSQL integration test is never treated as database verification.
- [Earlier runtime verification](../verification.md) and [advanced pipeline](../advanced-invoice-pipeline.md): recorded smoke tests and their limitations.

The publication diagrams explain runtime responsibilities; they are not benchmark outputs. The pipeline infographic is illustrative, while the architecture SVG and Mermaid source remain editable. The PDF is built from `article.md` with `build_article.py` and checked to contain exactly three pages.

## Boundaries before deployment

Production startup is disabled. Remaining work includes hosted identity hardening, multi-user roles, recovery, rate limits, operational backups, a persisted reversal interface and cent-level rounding policy. The US demo chart does not provide jurisdiction/tax applicability decisions or certified compliance. Pricing has no billing integration. See the [implementation status](../implementation-status.md).

The project's Apache-2.0 license covers original code and documentation; embedded third-party files retain their notices. Model and dataset terms are separate. Contributors should not upload private invoices, credentials, model weights or unlicensed datasets.
