-- name: GetOrganization :one
SELECT id::text, name, currency, accounting_standard, inventory_method
FROM identity.organizations WHERE id = $1;
