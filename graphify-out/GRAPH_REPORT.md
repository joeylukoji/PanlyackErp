# Graph Report - PanlyackERP  (2026-10-13)

## Corpus Check
- Corpus is ~16,827 words - fits in a single context window. You may not need a graph.

## Summary
- 346 nodes · 825 edges · 19 communities (12 shown, 7 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 25 edges (avg confidence: 0.91)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Documents module
- Inventory module
- Architecture and platform
- Architecture and platform
- Documents module
- Reporting module
- Architecture and platform
- Accounting module
- Architecture and platform
- Documents module
- Architecture and platform
- Architecture and platform
- Architecture and platform
- Documents module
- Architecture and platform
- Architecture and platform
- Architecture and platform

## God Nodes (most connected - your core abstractions)
1. `Server` - 29 edges
2. `Invoice` - 17 edges
3. `ExtractionRequest` - 12 edges
4. `Invoice` - 11 edges
5. `Store` - 11 edges
6. `Service` - 11 edges
7. `Planned: architecture blueprint` - 11 edges
8. `Entry` - 10 edges
9. `Service` - 10 edges
10. `Extractor` - 9 edges

## Surprising Connections (you probably didn't know these)
- `Complete document-to-posting MVP delivery plan` --semantically_similar_to--> `Planned: Invoice Posting Workflow`  [INFERRED] [semantically similar]
  tasks/plan.md → docs/architecture/blueprint.md
- `Decimal strings at boundaries and exact domain arithmetic` --semantically_similar_to--> `Exact money and quantities`  [INFERRED] [semantically similar]
  tasks/plan.md → AGENTS.md
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/erp/main.go → internal/bootstrap/app.go
- `run()` --calls--> `Load()`  [EXTRACTED]
  cmd/erp/main.go → internal/platform/config/config.go
- `Reviewed documents become inventory, accounting and cash operations` --conceptually_related_to--> `Mandatory human confirmation of extraction`  [EXTRACTED]
  PRODUCT.md → AGENTS.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Planned: one transaction coordinates invoice posting effects** — docs_architecture_blueprint_invoiceposting, docs_architecture_blueprint_documents, docs_architecture_blueprint_inventory, docs_architecture_blueprint_accounting, docs_architecture_blueprint_cash, docs_architecture_blueprint_unitofwork, docs_architecture_blueprint_audit_log, docs_architecture_blueprint_transactional_outbox [EXTRACTED 1.00]

## Communities (19 total, 7 thin omitted)

### Community 0 - "Documents module"
Cohesion: 0.08
Nodes (42): main(), run(), transactionKey, go_pkg_context, go_pkg_crypto_hmac, go_pkg_crypto_rand, go_pkg_crypto_sha256, go_pkg_embed (+34 more)

### Community 1 - "Inventory module"
Cohesion: 0.08
Nodes (27): Command, Log, Command, Executor, context.Context, github.com/jackc/pgx/v5/pgconn.CommandTag, github.com/jackc/pgx/v5/pgxpool.Pool, github.com/shopspring/decimal.Decimal (+19 more)

### Community 2 - "Architecture and platform"
Cohesion: 0.13
Nodes (28): DemoExtractor, Extractor, OllamaExtractor, validate_file(), authorize(), extract(), get_extractor(), live() (+20 more)

### Community 3 - "Architecture and platform"
Cohesion: 0.07
Nodes (36): Transactional and idempotent posting, Exact money and quantities, Mandatory human confirmation of extraction, Immutable posted entries and reversal corrections, Module isolation and public-port orchestration, Organization-scoped business queries, Project working agreement, Planned: Accounting (+28 more)

### Community 4 - "Documents module"
Cohesion: 0.12
Nodes (13): Client, Audit, Extractor, Storage, UnitOfWork, net/http.Client, Repository, Repository (+5 more)

### Community 5 - "Reporting module"
Cohesion: 0.11
Nodes (16): go_pkg_github_com_shopspring_decimal, Repository, Service, Accounting, Audit, Cash, Documents, Identity (+8 more)

### Community 6 - "Architecture and platform"
Cohesion: 0.35
Nodes (7): github.com/a-h/templ.Component, net/http.Request, net/http.ResponseWriter, csrf(), Server, invoiceID(), safeMessage()

### Community 7 - "Accounting module"
Cohesion: 0.30
Nodes (8): FrenchPCG, Operation, PostingEngine, USGAAP, build(), Entry, Line, Payment

### Community 8 - "Architecture and platform"
Cohesion: 0.14
Nodes (13): dependencies, htmx.org, devDependencies, tailwindcss, @tailwindcss/cli, name, private, scripts (+5 more)

### Community 9 - "Documents module"
Cohesion: 0.17
Nodes (10): Totals, go_pkg_regexp, testing.T, TestPostingPolicies(), TestReviewRequiredAndTotalExact(), Currency(), Parse(), TestExactDecimal() (+2 more)

### Community 10 - "Architecture and platform"
Cohesion: 0.33
Nodes (6): go_pkg_bytes, go_pkg_github_com_a_h_templ_generator, go_pkg_github_com_a_h_templ_parser_v2, go_pkg_go_format, main(), run()

### Community 12 - "Architecture and platform"
Cohesion: 0.50
Nodes (4): Configured AI service (demo extraction by default), Persistent documents volume mounted at /data/documents, Configured ERP service, Configured PostgreSQL 17 service

## Knowledge Gaps
- **25 isolated node(s):** `panlyackerp-ai`, `panlyackerp`, `transactionKey`, `csrfKey`, `name` (+20 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 68 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **7 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Server` connect `Architecture and platform` to `Documents module`, `Inventory module`, `Documents module`, `Reporting module`, `Documents module`?**
  _High betweenness centrality (0.004) - this node is a cross-community bridge._
- **Why does `New()` connect `Documents module` to `Inventory module`, `Architecture and platform`?**
  _High betweenness centrality (0.002) - this node is a cross-community bridge._
- **Why does `Invoice` connect `Documents module` to `Documents module`?**
  _High betweenness centrality (0.002) - this node is a cross-community bridge._
- **What connects `panlyackerp-ai`, `panlyackerp`, `transactionKey` to the rest of the system?**
  _25 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Documents module` be split into smaller, more focused modules?**
  _Cohesion score 0.08333333333333333 - nodes in this community are weakly interconnected._
- **Should `Inventory module` be split into smaller, more focused modules?**
  _Cohesion score 0.07609427609427609 - nodes in this community are weakly interconnected._
- **Should `Architecture and platform` be split into smaller, more focused modules?**
  _Cohesion score 0.12612612612612611 - nodes in this community are weakly interconnected._

## Coverage limitation
Graph generated from the initial implementation snapshot. Later UI and fiscal-period edits may not appear. Blueprint nodes describe requirements, not proof of implementation. Semantic extraction usage was not reported by the interrupted agent; zero above is an unavailable counter, not a measured zero.
