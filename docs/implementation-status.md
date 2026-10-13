# Implementation status

## Delivered and exercised
- Local Go app, PostgreSQL, migrations, FastAPI, health endpoints, timeouts, logs and shutdown.
- Seeded USD organization, FR PCG policy, FIFO and opening balances with balanced opening journal.
- File type/size checks, generated storage keys, SHA-256, image/PDF preview.
- Explicit demo extraction; separately configurable Ollama vision adapter.
- Editable invoice header, lines, taxes and split payment amounts; exact total validation.
- Human confirmation; explicit reopen transition; immutable posted state.
- Purchase receipt, sale issue, FIFO and AVCO cost allocation; product matching by confirmed SKU.
- Balanced purchase/sale/tax/COGS entries; credit and partial payment support at invoice posting.
- Cash/bank/mobile/card accounts and insufficient-funds rejection.
- Atomic posting, concurrent retry idempotency, audit, outbox record and synchronous reporting projection.
- Dashboard, invoices, inventory, journal and cash screens; responsive layouts; CSRF and local-host gate.
- Generated sqlc organization query. Other module SQL currently remains explicit parameterized pgx in module-owned adapters.

## Blueprint work still pending
- Production identity, Argon2id login, organization memberships, authorization and deployment hardening.
- Real vision model runtime verification; PDF rasterization/OCR; field-level extraction confidence UX; extraction retry queue.
- Independent product/category/warehouse CRUD, manual inventory adjustments, returns and losses.
- User-configurable organization/valuation policy and US chart of accounts; fiscal-year management UI.
- Standalone customer/supplier payments after posting, bank reconciliation, general expenses.
- Persisted reversal/correction workflow (the pure reversal rule is tested, but no reversal command/UI is delivered).
- General ledger and trial-balance screens, period filters, revenue/cashflow series, low-stock alerts and top-product projections.
- Broader `/api/v1` surface: only dashboard JSON currently exists.
- Fully generated sqlc adapters for all module queries; normalized invoice line/payment tables (draft invoice aggregate uses JSONB).
- Async outbox dispatcher/replay, multi-user concurrency UX, backups and operational monitoring.
- Ask PanlyackERP assistant, deliberately after the deterministic ERP core.

This list describes remaining work honestly; a working invoice MVP is not equivalent to all 70 blueprint sections being implemented.
