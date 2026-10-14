# PanlyackERP

A local ERP MVP that turns a reviewed invoice into inventory, journal and cash operations in one PostgreSQL transaction.

Go + Chi · templ + HTMX + Tailwind · PostgreSQL · FastAPI

**Status:** the purchase invoice demo and later settlement flow support self-service local test accounts. Each registration creates an isolated organization with its own opening balances. This is not a production SaaS release. The broader blueprint includes capabilities still outside this delivery; see [implementation status](docs/implementation-status.md).

## Quick start with Docker

1. Copy `.env.example` to `.env`. Replace the three secret placeholders with random values. Set `DATABASE_URL` to use the same database password.
2. Start Docker Desktop.
3. Run:

```sh
docker compose --env-file .env -f deployments/compose.yaml up --build -d
```

Open http://127.0.0.1:8080/register to create an isolated test account. To retain the pre-seeded Panlyack Trading workspace, create its first owner account from PowerShell and then sign in at http://127.0.0.1:8080/login:

```powershell
./scripts/create-owner-docker.ps1
```

The script prompts for a password. The ERP container applies Goose migrations before starting. PostgreSQL and documents use named volumes. Ports bind to loopback only.

## Local Windows development

Prerequisites: Go, Python 3.12+, Node.js and Docker Desktop. Assets and generated templ files are committed, so Node is only required for CSS changes. The container targets Go 1.27.2; the local build was verified with Go 1.26.4 because the 1.27 toolchain download failed on this network.

```powershell
docker compose --env-file .env -f deployments/compose.yaml up -d postgres
python -m venv .venv
.venv/Scripts/python -m pip install -e './ai-service[test]'
./scripts/start-local.ps1
./scripts/create-owner.ps1
```

The start script loads `.env`, applies migrations, builds Go, and starts Go/FastAPI in hidden background processes. It writes process IDs and logs under `var/`. Open http://127.0.0.1:8080/register to create a test workspace; no owner script is needed for this path. The owner script remains available for the pre-seeded Panlyack Trading workspace and can rotate its password and revoke old sessions. Stop the local services with `./scripts/stop-local.ps1`.

For a foreground Go development process after starting the AI service, use `./scripts/dev.ps1`.

## Try the complete flow

1. Import `fixtures/invoice-53740.png` from **Factures → Importer une facture**.
2. The default `EXTRACTOR_MODE=demo` returns a clearly labeled fixed 14-line invoice. It does **not** inspect the uploaded document.
3. Correct the supplier, quantities, prices, taxes or payment amounts. Add/remove lines when needed. The total must equal the exact sum of lines and tax.
4. Choose **Confirmer les données**, then **Comptabiliser la facture**.
5. Open stock, accounting, cash and dashboard screens. A newly registered workspace starts with 5,000 USD cash and no stock; after posting it has **4,462.60 USD cash** and **537.40 USD stock**. The pre-seeded Panlyack Trading workspace includes 2,000 USD opening stock and therefore reaches **2,537.40 USD stock**.
6. To test a later payment, set the invoice's initial **Espèces** amount to `0` before confirmation. After posting, open the invoice, choose an amount and a payment method under **Solde de la facture**, then record it. The remaining payable or receivable, journal and cash balances update together.

Repeated posting of the same invoice returns success without duplicate effects. Re-uploading a file creates another draft: file hashes are retained for traceability, not used as business-level duplicate invoice detection.

Settlements can be partial and may be repeated until the outstanding balance reaches zero. Each submission carries a request ID so a retry does not create a second payment. An overpayment, insufficient cash balance or closed fiscal period leaves the invoice and financial records unchanged.

Confirmed invoices require **Reprendre la vérification** before editing. Posted invoices and journal entries are immutable through the application. Journal rows also have database mutation guards.

## Real extraction

The service includes an Ollama adapter for a locally hosted open-weight vision model:

```dotenv
EXTRACTOR_MODE=ollama
MODEL_BASE_URL=http://127.0.0.1:11434
MODEL_NAME=gemma3:4b
```

Install and serve a vision-capable model separately, then restart FastAPI. In Compose use `http://host.docker.internal:11434` for a host model server. This adapter accepts PNG/JPEG and renders PDFs of 1–5 pages as bounded PNG images before sending them to the model. Model responses are validated by Pydantic and financial values are validated again in Go. Extraction failures never silently fall back to demo data.

**Real model extraction has not been verified against a running model in this workspace.** The PDF rendering path was tested with a mock model response. No model weights were downloaded and no invoice was sent to a third-party model provider.

## Tests and generation

```powershell
./scripts/dev.ps1 -Check
# The integration test creates and drops its own isolated database.
$env:TEST_DATABASE_URL = $env:DATABASE_URL
go test ./internal/workflows/invoiceposting -v
.venv/Scripts/python -m pytest ai-service/tests -q
go run ./scripts/generate-templates
sqlc generate -f db/sqlc.yaml
npm ci
npm run build
```

`TEST_DATABASE_URL` must reference a PostgreSQL user allowed to create databases. Without it, the integration test explicitly skips. See [verification](docs/verification.md) for checks actually run.

## Repository map

```text
cmd/erp, cmd/migrate       startup and Goose migrations
internal/bootstrap        explicit dependency wiring
internal/modules/
  identity                organization policy and generated sqlc query
  documents               upload, extraction, review and transitions
  inventory               products, FIFO/AVCO and stock persistence
  accounting              deterministic entries, fiscal periods, journal
  cash                    split payments, later settlements and cash movements
  reporting               transactional dashboard projection
internal/workflows/       cross-module invoice posting, settlement and registration
internal/platform/        config, transaction context, exact values, audit
internal/web/             HTTP handlers and local-demo security
web/                      templ components and embedded static assets
db/                       module schemas, Goose migrations, sqlc config
ai-service/               bounded extraction service, no ERP DB access
fixtures/                 synthetic invoice for reproducible demo
docs/                     architecture decisions, status and verification
graphify-out/             generated architecture graph, with coverage caveats
```

## Important boundaries

- Organization scope comes from the authenticated session in every business request. Each self-service registration creates one isolated organization and one owner account; the original seeded organization remains available through the owner script.
- Private pages and the dashboard API require a server-side session. Passwords are hashed with Argon2id and session tokens are stored as hashes. Registration provisions cash, accounting and reporting data atomically, including a 5,000 USD synthetic opening balance. This local release still refuses production mode; invited teammates, role permissions, password recovery, rate limits and HTTPS deployment are pending.
- Production mode refuses startup. Do not expose these local-demo ports through a tunnel or a public reverse proxy.
- Financial amounts use decimal strings and `shopspring/decimal`; quantities have six decimal places, money four. UI amounts display two places; stored amounts retain four.
- The seeded fiscal year is 2026. Missing or closed fiscal periods reject posting.
- Accounting policies currently share perpetual-inventory arithmetic. The demo uses a minimal FR PCG role mapping. US GAAP arithmetic is tested; a configurable US chart and country-specific tax compliance are not implemented.
- No stock or accounting module imports another business module. Workflow ports coordinate them. PostgreSQL transactions are resolved by adapters through context.

## Skills used

Ponytail and Graphify were already installed. `golang-design-patterns`, `golang-code-style`, `fastapi-expert` and `impeccable` were installed from the requested repositories. They informed the module boundaries, Go conventions, Python schemas and UI review. See [skills and decisions](docs/adr/0001-mvp-boundaries.md).
