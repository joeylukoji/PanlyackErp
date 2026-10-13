-- +goose Up
CREATE SCHEMA identity;
CREATE SCHEMA documents;
CREATE SCHEMA inventory;
CREATE SCHEMA accounting;
CREATE SCHEMA cash;
CREATE SCHEMA reporting;
CREATE SCHEMA platform;

CREATE TABLE identity.organizations (
 id uuid PRIMARY KEY, name text NOT NULL, currency text NOT NULL,
 accounting_standard text NOT NULL CHECK (accounting_standard IN ('FR_PCG','US_GAAP')),
 inventory_method text NOT NULL CHECK (inventory_method IN ('FIFO','AVCO'))
);
CREATE TABLE documents.invoices (
 organization_id uuid NOT NULL, id uuid NOT NULL, document_id uuid NOT NULL,
 status text NOT NULL CHECK (status IN ('NEEDS_REVIEW','CONFIRMED','POSTED')),
 data jsonb NOT NULL, version integer NOT NULL DEFAULT 1,
 storage_key text NOT NULL, mime_type text NOT NULL, sha256 text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(organization_id,id)
);
CREATE TABLE inventory.products (
 organization_id uuid NOT NULL, id uuid NOT NULL, sku text NOT NULL, name text NOT NULL,
 unit text NOT NULL, quantity numeric(20,6) NOT NULL DEFAULT 0 CHECK(quantity>=0),
 value numeric(20,4) NOT NULL DEFAULT 0 CHECK(value>=0),
 reorder_level numeric(20,6) NOT NULL DEFAULT 5,
 PRIMARY KEY(organization_id,id), UNIQUE(organization_id,sku)
);
CREATE TABLE inventory.stock_lots (
 organization_id uuid NOT NULL, id uuid NOT NULL, product_id uuid NOT NULL,
 quantity numeric(20,6) NOT NULL CHECK(quantity>=0), unit_cost numeric(20,10) NOT NULL CHECK(unit_cost>=0),
 received_at timestamptz NOT NULL DEFAULT now(), source_id uuid NOT NULL,
 PRIMARY KEY(organization_id,id), FOREIGN KEY(organization_id,product_id) REFERENCES inventory.products
);
CREATE TABLE inventory.stock_movements (
 organization_id uuid NOT NULL, id uuid NOT NULL, product_id uuid NOT NULL,
 quantity numeric(20,6) NOT NULL, value numeric(20,4) NOT NULL,
 source_type text NOT NULL, source_id uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,id), FOREIGN KEY(organization_id,product_id) REFERENCES inventory.products
);
CREATE INDEX ON inventory.stock_lots(organization_id,product_id,received_at);
CREATE TABLE accounting.accounts (
 organization_id uuid NOT NULL, role text NOT NULL, code text NOT NULL, name text NOT NULL,
 PRIMARY KEY(organization_id,role), UNIQUE(organization_id,code)
);
CREATE TABLE accounting.journal_entries (
 organization_id uuid NOT NULL, id uuid NOT NULL, source_type text NOT NULL, source_id uuid NOT NULL,
 reference text NOT NULL, entry_date date NOT NULL, currency text NOT NULL,
 status text NOT NULL DEFAULT 'POSTED' CHECK(status='POSTED'), created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,id), UNIQUE(organization_id,source_type,source_id)
);
CREATE TABLE accounting.journal_lines (
 organization_id uuid NOT NULL, id uuid NOT NULL, entry_id uuid NOT NULL, role text NOT NULL,
 debit numeric(20,4) NOT NULL CHECK(debit>=0), credit numeric(20,4) NOT NULL CHECK(credit>=0),
 CHECK((debit>0 AND credit=0) OR (credit>0 AND debit=0)), PRIMARY KEY(organization_id,id),
 FOREIGN KEY(organization_id,entry_id) REFERENCES accounting.journal_entries,
 FOREIGN KEY(organization_id,role) REFERENCES accounting.accounts
);
-- +goose StatementBegin
CREATE FUNCTION accounting.reject_posted_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Posted accounting records are immutable'; END;
$$;
-- +goose StatementEnd
CREATE TRIGGER immutable_entries BEFORE UPDATE OR DELETE ON accounting.journal_entries FOR EACH ROW EXECUTE FUNCTION accounting.reject_posted_mutation();
CREATE TRIGGER immutable_lines BEFORE UPDATE OR DELETE ON accounting.journal_lines FOR EACH ROW EXECUTE FUNCTION accounting.reject_posted_mutation();
CREATE TABLE cash.accounts (
 organization_id uuid NOT NULL, method text NOT NULL CHECK(method IN ('CASH','BANK','MOBILE_MONEY','CARD')),
 balance numeric(20,4) NOT NULL DEFAULT 0, PRIMARY KEY(organization_id,method)
);
CREATE TABLE cash.movements (
 organization_id uuid NOT NULL, id uuid NOT NULL, method text NOT NULL, amount numeric(20,4) NOT NULL,
 source_type text NOT NULL, source_id uuid NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(organization_id,id), UNIQUE(organization_id,source_type,source_id,method),
 FOREIGN KEY(organization_id,method) REFERENCES cash.accounts
);
CREATE TABLE reporting.summary (
 organization_id uuid PRIMARY KEY, revenue numeric(20,4) NOT NULL DEFAULT 0,
 purchases numeric(20,4) NOT NULL DEFAULT 0, cogs numeric(20,4) NOT NULL DEFAULT 0,
 cash numeric(20,4) NOT NULL DEFAULT 0, inventory numeric(20,4) NOT NULL DEFAULT 0,
 receivables numeric(20,4) NOT NULL DEFAULT 0, payables numeric(20,4) NOT NULL DEFAULT 0,
 posted_count integer NOT NULL DEFAULT 0
);
CREATE TABLE platform.audit_log (
 id uuid PRIMARY KEY, organization_id uuid NOT NULL, actor_id text NOT NULL,
 action text NOT NULL, entity_id uuid NOT NULL, details jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE platform.outbox_events (
 id uuid PRIMARY KEY, organization_id uuid NOT NULL, event_type text NOT NULL,
 source_id uuid NOT NULL, payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 projected_at timestamptz, UNIQUE(organization_id,event_type,source_id)
);
CREATE INDEX ON documents.invoices(organization_id,created_at DESC);
CREATE INDEX ON platform.audit_log(organization_id,entity_id);

-- +goose Down
DROP SCHEMA platform CASCADE;
DROP SCHEMA reporting CASCADE;
DROP SCHEMA cash CASCADE;
DROP SCHEMA accounting CASCADE;
DROP SCHEMA inventory CASCADE;
DROP SCHEMA documents CASCADE;
DROP SCHEMA identity CASCADE;
