# PanlyackERP

A local ERP MVP that turns a reviewed invoice into inventory, journal and cash operations in one PostgreSQL transaction.

Go + Chi · templ + HTMX + Tailwind · PostgreSQL · FastAPI

**Status:** the complete purchase invoice demo flow runs behind a local owner login. This is a single-organization MVP, not a production SaaS release. The broader blueprint includes capabilities still outside this delivery; see [implementation status](docs/implementation-status.md).

## Quick start with Docker

1. Copy `.env.example` to `.env`. Replace the three secret placeholders with random values. Set `DATABASE_URL` to use the same database password.
2. Start Docker Desktop.
3. Run:

```sh
docker compose --env-file .env -f deployments/compose.yaml up --build -d
```

Create the first owner account from PowerShell, then open http://127.0.0.1:8080/login:

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

The start script loads `.env`, applies migrations, builds Go, and starts Go/FastAPI in hidden background processes. It writes process IDs and logs under `var/`. The owner script prompts for an email/password and can be rerun to rotate the password and revoke old sessions. Open http://127.0.0.1:8080/login. Stop the local services with `./scripts/stop-local.ps1`.

For a foreground Go development process after starting the AI service, use `./scripts/dev.ps1`.

## Try the complete flow

1. Import `fixtures/invoice-53740.png` from **Factures → Importer une facture**.
2. The default `EXTRACTOR_MODE=demo` returns a clearly labeled fixed 14-line invoice. It does **not** inspect the uploaded document.
3. Correct the supplier, quantities, prices, taxes or payment amounts. Add/remove lines when needed. The total must equal the exact sum of lines and tax.
4. Choose **Confirmer les données**, then **Comptabiliser la facture**.
5. Open stock, accounting, cash and dashboard screens. Starting cash 5,000 USD and stock 2,000 USD become **4,462.60 USD** and **2,537.40 USD**.

Repeated posting of the same invoice returns success without duplicate effects. Re-uploading a file creates another draft: file hashes are retained for traceability, not used as business-level duplicate invoice detection.

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
  cash                    split payments and cash movements
  reporting               transactional dashboard projection
internal/workflows/       cross-module invoice posting
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

- Organization scope is explicit in business queries. The local UI currently uses one seeded organization and one owner role.
- The owner account belongs to the seeded organization. Private pages and the dashboard API require a server-side session. Passwords are hashed with Argon2id and session tokens are stored as hashes. This local release still refuses production mode; multi-user roles, password recovery and HTTPS deployment are pending.
- Production mode refuses startup. Do not expose these local-demo ports through a tunnel or a public reverse proxy.
- Financial amounts use decimal strings and `shopspring/decimal`; quantities have six decimal places, money four. UI amounts display two places; stored amounts retain four.
- The seeded fiscal year is 2026. Missing or closed fiscal periods reject posting.
- Accounting policies currently share perpetual-inventory arithmetic. The demo uses a minimal FR PCG role mapping. US GAAP arithmetic is tested; a configurable US chart and country-specific tax compliance are not implemented.
- No stock or accounting module imports another business module. Workflow ports coordinate them. PostgreSQL transactions are resolved by adapters through context.

## Skills used

Ponytail and Graphify were already installed. `golang-design-patterns`, `golang-code-style`, `fastapi-expert` and `impeccable` were installed from the requested repositories. They informed the module boundaries, Go conventions, Python schemas and UI review. See [skills and decisions](docs/adr/0001-mvp-boundaries.md).
