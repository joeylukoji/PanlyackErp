-- +goose Up
ALTER TABLE reporting.summary ADD COLUMN expenses numeric(20,4) NOT NULL DEFAULT 0;
INSERT INTO accounting.accounts(organization_id,role,code,name)
SELECT id,'OPERATING_EXPENSE',CASE WHEN accounting_standard='US_GAAP' THEN '6100' ELSE '628' END,'Operating expenses'
FROM identity.organizations;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM accounting.journal_lines WHERE role='OPERATING_EXPENSE') THEN
  RAISE EXCEPTION 'Posted expense entries prevent removal of their account mapping';
 END IF;
END $$;
-- +goose StatementEnd
DELETE FROM accounting.accounts WHERE role='OPERATING_EXPENSE';
ALTER TABLE reporting.summary DROP COLUMN expenses;
