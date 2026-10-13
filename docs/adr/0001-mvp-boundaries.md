# ADR 0001: a small local invoice MVP

Status: implemented, with explicit follow-up work.

The user selected the complete invoice flow rather than the phase-0-only prompt embedded in the source blueprint. We preserve the modular monolith and implement one complete purchase/sale posting path before adding the broader ERP catalogue.

## Decisions
- Pure decimal rules stay in Go domains. Small consumer-owned interfaces isolate the posting workflow from persistence. No ORM, event framework or generic repository.
- The invoice draft is an aggregate stored in JSONB with separate organization, status, version, provenance and timestamps. This reduces early synchronization complexity. Normalize lines/payments when analytical or cross-document queries justify it.
- Inventory, accounting and cash use normalized tables in separate schemas. The workflow owns one transaction; adapters resolve the transaction from context.
- Invoice row locks serialize retries. Module writes, audit, outbox and reporting projection commit atomically. Unique source keys give additional protection for journal/cash records.
- The outbox event is retained and projected synchronously in that transaction. No background broker is required for this local monolith. A later dispatcher needs explicit replay/version semantics.
- Demo access binds to loopback and rejects unknown Host values. Production startup is disabled until identity and access control exist. Local CSRF protection still applies.
- One 2026 demo fiscal period and minimal FR PCG chart support the demo. FR/US strategies share perpetual-inventory arithmetic, but account mappings and statutory compliance require additional configuration and review.
- Server-rendered templ + HTMX keeps state in Go. Tailwind builds local CSS; HTMX and CSS are embedded to avoid CDN dependencies.
- templ generation uses the official parser/generator libraries directly, avoiding its optional CLI proxy/LSP dependency downloads. Generated Go files are committed.
- sqlc 1.30.0 generates the organization query. The remaining parameterized pgx SQL is module-owned; migrating it to sqlc is tracked rather than falsely labeled generated.
- Go 1.27.2 is the container target. The Windows environment has Go 1.26.4; a failed official toolchain download required preserving local compatibility. Local tests report the actual version used.

## Requested skills
Ponytail: minimum complete flow and no speculative frameworks. Go skills: explicit wiring, errors, bounded resources and pure domains. FastAPI Expert: typed schemas, async adapter, dependency injection and tests. Impeccable: task-focused UI, sober visual hierarchy, responsive verification and mechanical detector. Graphify: architecture navigation with graph-health warnings retained.

The Impeccable auxiliary launcher initially failed during download, later became available, and its detector reported no mechanical findings. We continued from the explicit task and blueprint layout; no marketing-style image generation was needed.

## References
- [Ollama structured generation API](https://docs.ollama.com/api/generate)
- [sqlc configuration](https://docs.sqlc.dev/en/stable/reference/config.html)
- Local blueprint: `docs/architecture/blueprint.md`
