# Verification record

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

## Limits
- Real Ollama model extraction not exercised; adapter must be tested with the chosen installed model.
- PDF rendering was exercised with generated blank pages and a mocked model response; invoice recognition quality on real PDF documents is unverified.
- Full Docker image build, Go 1.27 runtime, race detector and dependency vulnerability audit are not yet verified.
- The AI image build was attempted but stopped after its dependency download stalled; the new PDF packages installed and passed tests in the local Python environment.
- Graphify output covers an earlier implementation snapshot. Its health report retains dangling references and collapsed edges; use it for navigation, not proof of correctness or completeness.
- This is a local demo with an owner login, not production-ready authentication. Rate limits, multi-user roles, password recovery and HTTPS deployment remain pending.
- Python tests pass with one upstream Starlette TestClient deprecation warning about httpx.
