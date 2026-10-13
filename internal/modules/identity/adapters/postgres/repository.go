package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgtype"
	"panlyackerp/internal/modules/identity/adapters/postgres/identitysql"
	"panlyackerp/internal/modules/identity/domain"
	"panlyackerp/internal/platform/db"
)

type Repository struct{ DB db.Store }

func (r Repository) Get(ctx context.Context, id string) (domain.Organization, error) {
	var key pgtype.UUID
	if err := key.Scan(id); err != nil {
		return domain.Organization{}, err
	}
	row, err := identitysql.New(r.DB.Conn(ctx)).GetOrganization(ctx, key)
	if err != nil {
		return domain.Organization{}, err
	}
	o := domain.Organization{ID: row.ID, Name: row.Name, Currency: row.Currency,
		AccountingStandard: row.AccountingStandard, InventoryMethod: row.InventoryMethod}
	return o, o.Validate()
}
