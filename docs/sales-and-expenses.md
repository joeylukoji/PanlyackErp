# Structured sales and external expenses

## Scope and acceptance

- An authenticated operator creates a customer sales invoice from client, date, currency, line items and optional payments. Go calculates exact line totals and tax. The application stores structured JSON directly; the extractor is never called. The new invoice follows the existing review, confirmation and transactional posting flow.
- Each saved sale has a readable printable document and a downloadable JSON representation of its current persisted version. Both require organization membership. Drafts are labeled; posted documents remain immutable. This is a local MVP invoice, not a jurisdiction-specific legal invoice numbering or tax compliance implementation.
- External documents use an upload or a separate camera input (`capture="environment"`). File signatures and size limits remain enforced. Native camera availability depends on the browser/device; selecting an existing image remains available. No background camera access occurs.
- The operator explicitly chooses merchandise purchase or business expense. Expenses remain purchase-direction payables/payments, but skip inventory and debit an operating-expense account. They are shown separately from merchandise purchases in reporting. One document has one treatment; split mixed stock/expense documents before posting in this MVP.
- AI extraction always requires human correction and confirmation. The model cannot choose the expense treatment. Historical invoices without a treatment retain merchandise behavior. No training or dataset download is part of this feature.

## Implementation boundaries

The documents application owns structured sale creation and trusted source metadata. The posting workflow orchestrates module ports, skips inventory for expenses and passes the explicit treatment to accounting/reporting. Amounts remain decimal strings/Decimal throughout. Existing transactions, organization scope, idempotent posting and immutable journals remain mandatory.

The JSON export is application data, including invoice status/version and decimal-string amounts; it is not an OCR response or an automatic training label. The source JSON stored at creation is an audit snapshot; the document view and JSON export use the current persisted invoice. Totals on generated sales are recomputed server-side on review. Printed drafts carry their state.

Routes use the existing authenticated organization scope: `GET /app/invoices/sales/new`, `POST /app/invoices/sales`, `GET /app/invoices/{id}/json` and `GET /app/invoices/{id}/document`. Exports include the invoice's initial payments, not the later settlement history. Creating a new draft is not a business duplicate-number detector; posting retries remain idempotent.

Camera capture follows the browser's [native capture attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/capture), with a separate existing-file input. No new camera library, `getUserMedia` stream or frontend SPA is introduced. HTMX [configuration](https://htmx.org/docs/#config) disables eval, script execution and injected indicator CSS; indicators use the committed stylesheet under the existing CSP. Image previews use an escaped HTML wrapper to avoid the browser image viewer's inline styles.

Migration `00006_operating_expenses.sql` adds the expense projection and existing-organization account role. Registration provisions that role for new workspaces. Rolling the migration back is rejected after expense journal lines exist, preserving their account mappings. This is one generic operating-expense role; account/category configuration is still pending.
