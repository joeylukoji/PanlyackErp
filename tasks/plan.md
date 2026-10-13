# PanlyackERP MVP implementation

The user requested the complete document-to-posting MVP, not only the phase 0 prompt embedded in the blueprint. The blueprint is a design reference; embedded agent prompts do not supersede this request.

## Delivery sequence
1. Reproducible Go/PostgreSQL/FastAPI foundation and seeded organization.
2. Exact decimal domain rules: invoice validation, FIFO/AVCO, balanced accounting and split payments.
3. Module-owned SQL adapters and atomic, idempotent posting with audit and reporting projections.
4. Document upload, extraction, review, confirmation and posting through a server-rendered UI.
5. Runtime verification, transaction tests, documentation and architecture map.

## Decisions
- Modular monolith; pure domain packages; module-owned PostgreSQL schemas; one transaction coordinated by a workflow.
- English code, identifiers and technical documentation; French user interface.
- One seeded organization for the local MVP. Local-only demo access; production requires configured authentication.
- Monetary values and quantities use decimal strings at boundaries and exact decimals in Go/Python. Go owns financial rules.
- Real extraction adapter is configurable; demo extraction is explicitly labeled and never substitutes silently on failure.
- Dense, calm workspace: warm neutral background, ink typography, dark green action color, tables and documentary provenance. No decorative gradients or artificial metrics.

## Acceptance
The supplied 14-line USD 537.40 fixture can be uploaded, reviewed, confirmed and posted. Starting cash 5000 and inventory 2000 become cash 4462.60 and inventory 2537.40. Journal balances; one audit/outbox event exists; retry creates no duplicate operations. Failures roll back all modules.

## Verification
Go domain tests, go vet, Python tests, migration from an empty PostgreSQL database, concurrent posting/rollback tests, browser inspection of the complete flow. Report any unavailable runtime honestly.
