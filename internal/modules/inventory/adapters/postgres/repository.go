package postgres

import (
	"context"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"panlyackerp/internal/modules/inventory/application"
	"panlyackerp/internal/modules/inventory/domain"
	"panlyackerp/internal/platform/db"
)

type Repository struct{ DB db.Store }

func (r Repository) FindOrCreate(ctx context.Context, org string, item application.Item, create bool) (application.Product, error) {
	conn := r.DB.Conn(ctx)
	if create {
		if _, err := conn.Exec(ctx, `INSERT INTO inventory.products(organization_id,id,sku,name,unit) VALUES($1,$2,$3,$4,$5) ON CONFLICT(organization_id,sku) DO NOTHING`, org, uuid.NewString(), item.SKU, item.Name, item.Unit); err != nil {
			return application.Product{}, err
		}
	}
	p := application.Product{}
	err := conn.QueryRow(ctx, `SELECT id::text,sku,name,unit,quantity::text,value::text FROM inventory.products WHERE organization_id=$1 AND sku=$2 FOR UPDATE`, org, item.SKU).Scan(&p.ID, &p.SKU, &p.Name, &p.Unit, &p.Quantity, &p.Value)
	return p, err
}

func (r Repository) Lots(ctx context.Context, org, product string) ([]domain.Lot, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT id::text,quantity::text,unit_cost::text FROM inventory.stock_lots WHERE organization_id=$1 AND product_id=$2 AND quantity>0 ORDER BY received_at,id FOR UPDATE`, org, product)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lots := []domain.Lot{}
	for rows.Next() {
		l := domain.Lot{}
		if err := rows.Scan(&l.ID, &l.Quantity, &l.Cost); err != nil {
			return nil, err
		}
		lots = append(lots, l)
	}
	return lots, rows.Err()
}

func (r Repository) Receive(ctx context.Context, org, source string, p application.Product, item application.Item) error {
	conn := r.DB.Conn(ctx)
	value := item.Quantity.Mul(item.Price).Round(4)
	if _, err := conn.Exec(ctx, `INSERT INTO inventory.stock_lots(organization_id,id,product_id,quantity,unit_cost,source_id) VALUES($1,$2,$3,$4,$5,$6)`, org, uuid.NewString(), p.ID, item.Quantity.String(), item.Price.String(), source); err != nil {
		return err
	}
	return r.move(ctx, org, source, p.ID, item.Quantity, value)
}

func (r Repository) Issue(ctx context.Context, org, source string, p application.Product, q, cost decimal.Decimal, lots []domain.Lot) error {
	for _, lot := range lots {
		if _, err := r.DB.Conn(ctx).Exec(ctx, `UPDATE inventory.stock_lots SET quantity=$3,unit_cost=$4 WHERE organization_id=$1 AND id=$2`, org, lot.ID, lot.Quantity.String(), lot.Cost.String()); err != nil {
			return err
		}
	}
	return r.move(ctx, org, source, p.ID, q.Neg(), cost.Neg())
}

func (r Repository) move(ctx context.Context, org, source, product string, q, v decimal.Decimal) error {
	conn := r.DB.Conn(ctx)
	if _, err := conn.Exec(ctx, `UPDATE inventory.products SET quantity=quantity+$3,value=value+$4 WHERE organization_id=$1 AND id=$2`, org, product, q.String(), v.String()); err != nil {
		return err
	}
	_, err := conn.Exec(ctx, `INSERT INTO inventory.stock_movements(organization_id,id,product_id,quantity,value,source_type,source_id) VALUES($1,$2,$3,$4,$5,'INVOICE',$6)`, org, uuid.NewString(), product, q.String(), v.String(), source)
	return err
}

func (r Repository) List(ctx context.Context, org string) ([]application.Product, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT id::text,sku,name,unit,quantity::text,value::text FROM inventory.products WHERE organization_id=$1 ORDER BY name LIMIT 200`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []application.Product{}
	for rows.Next() {
		p := application.Product{}
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.Unit, &p.Quantity, &p.Value); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
