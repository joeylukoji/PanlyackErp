# ADR 0003: Isolated local test registration

Status: Accepted for the local MVP.

## Context

The first login flow allowed only a CLI-created owner of the seeded Panlyack Trading organization. Sharing that organization with self-registered testers would expose their invoices and let one test change another tester's stock and balances.

## Decision

`GET /register` and `POST /register` let a tester create an organization name, email and password. One transaction creates a new organization, its owner membership and session, a FR PCG account chart, an open fiscal period, four payment accounts, a balanced synthetic 5,000 USD cash opening journal and the dashboard projection. Registration rolls back completely if an email is already registered or any provisioning step fails. The existing seeded organization and owner CLI remain available.

Every private HTTP handler now takes its organization ID from the verified server-side session. The UI displays the session's organization name. The original document, inventory, accounting, cash and reporting adapters continue to accept explicit organization IDs. The bundled demo invoice is dated 2026, so new workspaces open that fiscal period as well as the current year.

## Consequences

- Testers can import, review and post invoices independently. A paid purchase using the bundled synthetic invoice has enough opening cash to complete.
- An email identifies one local workspace. The owner CLI refuses to attach an existing account from another organization.
- This remains loopback-only demo access. Invitations, role management, password recovery, rate limiting and HTTPS deployment require a separate production identity design.
