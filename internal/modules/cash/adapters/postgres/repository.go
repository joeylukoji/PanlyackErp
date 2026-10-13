package postgres

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"panlyackerp/internal/modules/cash/ports"

	"panlyackerp/internal/platform/db"
)

type Repository struct{ DB db.Store }
type Payment = ports.Payment
type Command = ports.Command
type Account = ports.Account
type Movement = ports.Movement

func (r Repository) Record(ctx context.Context, c Command) error {
	for _, payment := range c.Payments {
		if payment.Amount.IsZero() {
			continue
		}
		if payment.Amount.IsNegative() {
			return errors.New("negative payment")
		}
		amount := payment.Amount
		if c.Direction == "PURCHASE" {
			amount = amount.Neg()
		}
		tag, err := r.DB.Conn(ctx).Exec(ctx, `UPDATE cash.accounts SET balance=balance+$3 WHERE organization_id=$1 AND method=$2 AND balance+$3>=0`, c.OrganizationID, payment.Method, amount.String())
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errors.New("insufficient funds in payment account")
		}
		_, err = r.DB.Conn(ctx).Exec(ctx, `INSERT INTO cash.movements(organization_id,id,method,amount,source_type,source_id) VALUES($1,$2,$3,$4,'INVOICE',$5)`, c.OrganizationID, uuid.NewString(), payment.Method, amount.String(), c.SourceID)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) Accounts(ctx context.Context, org string) ([]Account, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT method,balance::text FROM cash.accounts WHERE organization_id=$1 ORDER BY method`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Account{}
	for rows.Next() {
		a := Account{}
		if err := rows.Scan(&a.Method, &a.Balance); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}

func (r Repository) Movements(ctx context.Context, org string) ([]Movement, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT source_id::text,method,amount::text,created_at::date::text FROM cash.movements WHERE organization_id=$1 ORDER BY created_at DESC LIMIT 100`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Movement{}
	for rows.Next() {
		a := Movement{}
		if err := rows.Scan(&a.SourceID, &a.Method, &a.Amount, &a.Date); err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	return result, rows.Err()
}
