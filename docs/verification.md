# Verification record

Environment: Windows, Go 1.26.4, Python 3.13.5, PostgreSQL 17 in Docker. Container build targets Go 1.27.2 but that toolchain download did not complete locally.

## Passed
- `go test ./...` with PostgreSQL integration enabled.
- `go vet ./...`.
- `python -m pytest ai-service/tests -q`: authenticated demo extraction, 14 lines, exact total and invalid-file rejection.
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

## Limits
- Real Ollama model extraction not exercised; adapter must be tested with the chosen installed model.
- Full Docker image build, Go 1.27 runtime, race detector and dependency vulnerability audit are not yet verified.
- Graphify output covers an earlier implementation snapshot. Its health report retains dangling references and collapsed edges; use it for navigation, not proof of correctness or completeness.
- This is a local demo with no production authentication.
- Python tests pass with one upstream Starlette TestClient deprecation warning about httpx.
