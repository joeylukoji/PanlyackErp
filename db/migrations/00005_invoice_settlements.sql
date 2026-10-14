-- +goose Up
CREATE TABLE cash.invoice_payments (
 organization_id uuid NOT NULL,
 id uuid NOT NULL,
 invoice_id uuid NOT NULL,
 method text NOT NULL CHECK(method IN ('CASH','BANK','MOBILE_MONEY','CARD')),
 amount numeric(20,4) NOT NULL CHECK(amount>0),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,id)
);
CREATE INDEX ON cash.invoice_payments(organization_id,invoice_id,created_at);

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM cash.invoice_payments) THEN
  RAISE EXCEPTION 'Recorded invoice payments cannot be dropped';
 END IF;
END $$;
-- +goose StatementEnd
DROP TABLE cash.invoice_payments;
