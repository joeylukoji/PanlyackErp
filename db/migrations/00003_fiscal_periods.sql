-- +goose Up
CREATE TABLE accounting.fiscal_periods (
 organization_id uuid NOT NULL, year integer NOT NULL,
 closed boolean NOT NULL DEFAULT false, PRIMARY KEY(organization_id,year)
);
INSERT INTO accounting.fiscal_periods VALUES('11111111-1111-4111-8111-111111111111',2026,false);

-- +goose Down
DROP TABLE accounting.fiscal_periods;
