# Implementation status

## Delivered and exercised
- Local OCR+Gemma product-row checks and bounded corrective views; an experimental US account-role LoRA/RAG advisor with source links, rule comparison and human confirmation. New registrations offer US/FR demo charts. US nonrecoverable purchase tax is included in stock/expense cost; buyer-imposed collected sales tax is separated from revenue. See [scope, training results and limits](advanced-invoice-pipeline.md).
- Local Go app, PostgreSQL, migrations, FastAPI, health endpoints, timeouts, logs and shutdown.
- Seeded USD organization, FR PCG policy, FIFO and opening balances with balanced opening journal.
- File type/size checks, generated storage keys, SHA-256, image/PDF preview.
- Explicit demo extraction; separately configurable Ollama vision adapter with 1–5 page PDF rasterization.
- Installed Gemma 3 4B exercised through the real adapter; reproducible 50-document local pilot, annotation templates and exact-decimal scoring are available. Schema failures are recorded; extraction accuracy remains unmeasured without human references.
- A second 100-document experiment exercises explicit ISO date guidance and calendar validation. It yielded 96 schema-valid responses and 4 rejections, including an impossible month. Same-image acceptance improved from 4/50 to 48/50; these are not accuracy measurements, and mixed source document types/currencies remain a concern.
- Editable invoice header, lines, taxes and split payment amounts; exact total validation.
- Human confirmation; explicit reopen transition; immutable posted state.
- Purchase receipt, sale issue, FIFO and AVCO cost allocation; product matching by confirmed SKU.
- Balanced purchase/sale/tax/COGS entries; credit and partial payment support at invoice posting.
- Cash/bank/mobile/card accounts and insufficient-funds rejection.
- Atomic posting, concurrent retry idempotency, audit, outbox record and synchronous reporting projection.
- Dashboard, invoices, inventory, journal and cash screens; responsive layouts; CSRF and local-host gate.
- Generated sqlc organization query. Other module SQL currently remains explicit parameterized pgx in module-owned adapters.
- Local owner login, Argon2id password hashing, organization membership check, expiring server-side sessions and revocation on logout or password rotation.
- Self-service local registration creates an isolated organization, owner, session, accounting chart, fiscal period, cash accounts and synthetic opening balance in one transaction.
- Posted invoices support partial supplier payments and customer receipts with an immutable journal entry, cash movement, audit/outbox record and updated payable or receivable. Retries are idempotent and overpayments are rejected.
- The accounting screen now shows a trial balance with exact debit and credit totals for every account; selecting an account opens its ledger lines. Both views are organization-scoped, and the ledger displays the latest 400 lines.

## Blueprint work still pending
- Production identity: invited teammates, multi-user roles, password recovery, rate limits, HTTPS cookies and deployment hardening.
- OCR quality assessment against human references, field-level extraction confidence UX and extraction retry queue. The real-model pilot exposes date-format failures and mislabeled source documents that need review before training.
- Independent product/category/warehouse CRUD, manual inventory adjustments, returns and losses.
- Editing existing organization/valuation policies and charts; fiscal-year management UI. New registrations offer FR/US demo charts.
- Bank reconciliation and configurable expense categories/accounts; a single operating-expense treatment is delivered.
- Persisted reversal/correction workflow (the pure reversal rule is tested, but no reversal command/UI is delivered).
- Period filters, revenue/cashflow series, low-stock alerts and top-product projections. The general ledger is currently capped at 400 recent lines per selected account.
- Broader `/api/v1` surface: dashboard JSON and an authenticated per-invoice JSON download exist; no general external CRUD API is delivered.
- Fully generated sqlc adapters for all module queries; normalized invoice line/payment tables (draft invoice aggregate uses JSONB).
- Async outbox dispatcher/replay, multi-user concurrency UX, backups and operational monitoring.
- Ask PanlyackERP assistant, deliberately after the deterministic ERP core.

This list describes remaining work honestly; a working invoice MVP is not equivalent to all 70 blueprint sections being implemented.

## Local annotation workbench

A separate loopback FastAPI/HTMX tool now supports scan rotation, explicit document classification, partial decimal-string references, draft lines, named human confirmation, exclusions, stale-edit rejection and ZIP reference export. It writes ignored local annotations only; there is no ERP posting or training. See [annotation review](annotation-review.md). Actual human reference review remains pending.

## Structured sales and external expenses

The authenticated HTMX workspace now creates sales from entered client/line/payment data and computes exact totals without invoking AI. Persisted invoices have a JSON download; authored sales also have a printable view labeled by status. Uploaded supplier invoices and expenses accept a file or native camera input and retain human confirmation. Expenses skip inventory, debit an operating-expense role and update a separate expense aggregate. Posting remains transactional and idempotent. Invoice exports/documents are organization-scoped. See [feature scope](sales-and-expenses.md); legal numbering, customer master data and mixed stock/expense documents remain outside this slice.
