# Project working agreement

Read docs/architecture/blueprint.md and tasks/todo.md before changing architecture.
Code, identifiers and technical documentation are English. Keep user-facing progress in the user's language.
Use Ponytail for simplicity, golang-design-patterns and golang-code-style for Go, fastapi-expert for Python, impeccable for UI, and graphify for architecture navigation when a graph exists.
Implement complete vertical slices; no generic repositories, speculative frameworks, ORM or frontend SPA.
Business modules must not import each other. Workflows orchestrate their public ports. Domain code must not import HTTP or database drivers.
Money and quantity must never use binary floats. Posted entries are immutable; corrections require reversal. Extraction always requires human confirmation.
Every business query is organization-scoped. Posting must be transactional and idempotent.
Keep tests and docs current; never claim a check passed unless it ran.
