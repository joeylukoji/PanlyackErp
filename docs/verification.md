# Verification record

## US accounting RAG and OCR product pipeline — 2026-10-17

- `go test ./...`, `go vet ./...`, template generation and the new currency/policy page test passed. Registration and posting integration suites also passed with a live TEST_DATABASE_URL in their isolated databases, covering tax-inclusive US inventory/expense cost, buyer sales-tax liability, bank receipts, organization scope, rollback and idempotency.
- `.venv/Scripts/python -m pytest ai-service/tests -q`: 67 passed, with the existing Starlette/httpx deprecation warning. Includes retrieval partition isolation, policy rejection, bounded process invocation, corpus hash mismatch, inference-slot retention after timeouts, exact product/payment diagnostics and header-focused supplier correction without changing quantities or currency.
- Four-epoch local US role LoRA+RAG training completed on 184 synthetic examples: 46/46 validation role matches, peak CUDA allocation 2.654 GiB. Its 46 test examples and the 108 invoice test images remain unused. Repeated structural templates in retrieval make this a synthetic interpolation result, not an independent accounting accuracy result.
- Real HTTP checks exercised OCR/Gemma upload, explicit synthetic manual confirmation, read-only RAG/US-chart proposal, repeated purchase/sale posting and exact stock/cash/AP/AR balances in a new QA organization. A final actual isolated CPU worker also proposed the matching roles for a taxed, partially bank-paid sale including COGS (HTTP 200, 40.41 seconds).
- The actual registration page exposes the US/FR chart selector. Optional runtime configuration is active locally; no model/data download was resumed. See [implementation, artifacts and limitations](advanced-invoice-pipeline.md).
- Final real OCR/Gemma smoke: HTTP 200 in 25.48 seconds, correct issuer and EUR currency, all four product rows retained. Header-only correction cannot change product money/quantity/currency. A printed-cent rounding discrepancy remains visible. Tailwind build also passed.

Environment: Windows, Go 1.26.4, Python 3.13.5, PostgreSQL 17 in Docker. Container build targets Go 1.27.2 but that toolchain download did not complete locally.

## Passed
- `go test ./...` with PostgreSQL integration enabled.
- `go vet ./...`.
- `python -m pytest ai-service/tests -q`: four tests covering authenticated demo extraction, exact total, invalid files, two-page PDF rasterization to PNG images for a mocked Ollama response and the five-page limit.
- Goose migrations on the new demo database; isolated integration database created from all migrations.
- Five concurrent posting calls produce one journal, one posting audit and one financial effect.
- A late insufficient-funds error rolls back inventory and journal writes.
- Cross-organization invoice read fails; posted journal mutation fails.
- Credit sale recognizes revenue, receivable and FIFO COGS; closed fiscal period rejects posting.
- Browser: upload synthetic PNG through HTMX, reject total 537.41, correct to 537.40, edit supplier, confirm, post, inspect success and dashboard.
- Browser balances: cash 4462.60 USD, inventory 2537.40 USD, one posted invoice.
- Desktop 1440px and mobile 390px inspection; no JavaScript warning/error observed in the inspected flow.
- Tailwind production CSS build and sqlc organization query generation.
- Impeccable mechanical detector: no findings.
- Windows startup applies migrations, builds both-service launch configuration and checks readiness; stop removes the owned Python child process and Go process. Restart preserves the posted invoice and dashboard balances.
- Browser line editing: add a fifteenth line, remove it, save the 14-line draft successfully; no financial effect until posting.
- Owner authentication: Argon2id hash verification, organization membership, server-side session creation and revocation, rejected password and private-route redirect. Tested in isolated PostgreSQL integration and a local HTTP smoke flow.
- Updated desktop and mobile screens: login, dashboard and invoice review inspected at 1440px and 390px, with no document-wide horizontal overflow.
- New responsive navigation and images inspected at 1440px, 390px and 320px. The mobile drawer opens, closes from its backdrop, and all five navigation links reach their page; the dashboard import action reaches the upload page. A fresh browser session after restart had no warnings or errors.
- Impeccable mechanical detector reported no findings for the changed templ and CSS files.
- Registration: isolated PostgreSQL integration created two organizations and verified independent sessions, 12 accounting roles, open period, balanced opening journal, cash accounts, duplicate-email rollback and no cross-organization summary changes.
- Browser: a new local test account registered, imported the synthetic PNG, confirmed and posted it, viewed stock 537.40 USD and cash 4462.60 USD, then logged out and back into the same organization. Registration layout was inspected at 1440px and 390px without page overflow.
- Four user-supplied images load as local assets on authentication, dashboard, invoice import and accounting. Login, dashboard, import and journal were visually inspected at desktop width; registration, dashboard, import and journal were inspected at 390px with no document-wide horizontal overflow. The local registration form still creates a working session.
- After the image changes, templ generation, Tailwind CSS build, `go vet ./...`, PostgreSQL registration/posting integration tests and the Impeccable detector passed; the detector returned no findings.
- Later invoice settlement: PostgreSQL integration covers customer receipt, supplier payment, exact journal balance, retry idempotency, cross-organization rejection, overpayment, insufficient funds and a closed fiscal period. A local HTTP flow registered a fresh workspace, uploaded and confirmed a credit invoice, posted it, recorded 100 USD cash, retried the same request and verified a 437.40 USD payable with 4,900 USD cash.
- After the settlement change, templ generation, CSS build, `go test ./...` with PostgreSQL integration, `go vet ./...` and the Impeccable detector passed.
- After the final local restart, the invoice page displayed the precise 437.40 USD balance; a 500 USD overpayment returned a French validation error and left the 437.40 USD payable and 4,900 USD cash unchanged.

## Limits

Local extraction evaluation on 2026-10-16: `pytest ai-service/tests -q` passed 10 tests, including deterministic byte-duplicate-free sampling, reference overwrite protection, exact-decimal scoring, unconfirmed-reference exclusion, failed-prediction accounting (including empty reference line lists) and continuing a run after an extraction error. The real Gemma 3 4B baseline attempted all 50 selected scans: 4 schema-valid responses and 46 date-field validation failures. No human reference was confirmed, so field accuracy remains unmeasured. Dataset download was kept stopped, and no ERP data was posted.

Follow-up on the same date: the Python suite passed 15 tests after adding calendar-date rejection and valid-leap-day coverage. The OpenAPI issue-date string contract and description were checked. The 100-document date-guidance run completed with 96 schema-valid responses and 4 rejections (empty date, two invalid/truncated JSON responses, and an impossible month). The original 50 source hashes match the first 50 in the expanded set, with 48 accepted outputs on those same images versus the original 4. No human reference was confirmed; one assistant visual draft stays pending and excluded from scoring. The source download stayed stopped, and no fine-tuning or ERP posting occurred.

- Installed Gemma 3 4B has been exercised with real RVL-CDIP scans through the existing adapter. The 50-document pilot stores successes and failures under `var/evaluations/invoices-50/runs/`; this does not establish extraction accuracy without human-confirmed references. See [evaluation procedure](invoice-evaluation.md).
- PDF rendering was exercised with generated blank pages and a mocked model response; invoice recognition quality on real PDF documents is unverified.
- Full Docker image build, Go 1.27 runtime, race detector and dependency vulnerability audit are not yet verified.
- The AI image build was attempted but stopped after its dependency download stalled; the new PDF packages installed and passed tests in the local Python environment.
- Graphify output covers an earlier implementation snapshot. Its health report retains dangling references and collapsed edges; use it for navigation, not proof of correctness or completeness.
- This is a local demo with self-service owner registration, not production-ready authentication. Rate limits, invited teammates, password recovery and HTTPS deployment remain pending.
- Python tests pass with one upstream Starlette TestClient deprecation warning about httpx.
- Chrome DevTools MCP was unavailable for this settlement change; the updated form was verified through its rendered HTML and local HTTP flow, not a fresh visual browser inspection.

## Annotation workbench — 2026-10-16

- `pytest ai-service/tests -q`: 27 passed, with the existing upstream Starlette/httpx deprecation warning. Coverage includes classification, attribution, exact decimal strings, invalid dates, CSRF (including Unicode), origin/host rejection, bounded forms, path containment, stale edits, changed-image export rejection, draft lines and confirmation revocation.
- The PowerShell launcher started the actual 100-document workbench on loopback port 8091. Real annotations remain pending; no dataset download, model inference, training or ERP posting was launched.
- Chrome headless through CDP loaded real scans at 1440, 390 and 320 CSS pixels, with all images loaded and no document-wide horizontal overflow. Desktop and mobile screenshots were inspected. On a separate two-document fixture, browser actions verified rotation, adding a line, confirmation, reverting to draft and exclusion, with no JavaScript/runtime or browser log errors. Fixture confirmations are test actions, never human labels for the real corpus.
- The CUA browser tool failed to initialize (missing kernel asset path); local Chrome provided the runtime verification. Playwright installation was interrupted after its package download stalled; it is not a project dependency.
- Impeccable detector ran once and identified one thick accent border; that border was removed. The detector was not rerun.
- Export is a partial-reference ZIP, not a full supervised-training corpus. Human reference review and a genuinely held-out set remain required before accuracy or training claims.

## Structured sales and external expenses — 2026-10-16

- `go test ./...` ran with `TEST_DATABASE_URL` set; PostgreSQL posting and registration integration tests passed in isolated databases. `go vet ./...` passed. New tests cover exact authored totals, source/organization metadata, no extractor invocation for sales, explicit external treatment, expense journal roles, transaction rollback on insufficient funds, posting retries, absence of expense stock movements and cross-organization access. Registration now verifies 13 account roles.
- `pytest ai-service/tests -q`: 27 passed, with the existing upstream Starlette/httpx deprecation warning. No training or source dataset download ran.
- templ generation and Tailwind build passed. Impeccable detector ran once on the changed templ/CSS sources and returned `[]`.
- The real local HTTP flow created isolated test organizations, imported the synthetic stock fixture, created an 11.0000 USD structured sale while ignoring a forged client total/direction, confirmed/posted it, and verified an idempotent retry. A 100 USD expense uploaded through the camera field posted without changing inventory or merchandise purchases. Another organization's JSON/document requests returned 404; invalid image signatures and simultaneous file/photo inputs were rejected.
- Chrome headless/CDP inspected creation, import, sale review, expense review and the printable sale at 1440, 390 and 320 CSS pixels. All images loaded and no document-wide horizontal overflow remained. HTMX add/remove line, sale creation and camera-field upload worked. After fixing image-preview CSP errors and the standalone favicon, the confirmation batch reported zero runtime/browser-log errors. Screenshots and test sessions remain ignored under `var/`.
- This verifies file submission through the native camera input, not physical camera hardware. Real-phone camera launch is unverified. External extraction was tested in explicitly labeled demo mode; no real-model accuracy claim follows from these checks.
- Windows reserved ports 55425–55524, blocking the old PostgreSQL host port. Local `.env` now uses 15432 through the configurable `POSTGRES_PORT`; the existing named database volume was retained. The annotation workbench moved to 8093, leaving 8091 for extraction. The normal start script reported ERP/FastAPI readiness.
