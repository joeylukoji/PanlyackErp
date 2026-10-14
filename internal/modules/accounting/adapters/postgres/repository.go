package postgres

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"panlyackerp/internal/modules/accounting/ports"
	"panlyackerp/internal/platform/db"
	"time"
)

type Repository struct{ DB db.Store }
type Posting = ports.Posting
type JournalRow = ports.JournalRow
type BalanceRow = ports.BalanceRow

func (r Repository) SeedDemo(ctx context.Context, org, sourceID string) error {
	conn := r.DB.Conn(ctx)
	_, err := conn.Exec(ctx, `INSERT INTO accounting.accounts(organization_id,role,code,name)
		SELECT $1,role,code,name FROM (VALUES
		('CASH','531','Cash'),('BANK','512','Bank'),('MOBILE_MONEY','517','Mobile money'),
		('CARD','511','Card settlement'),('INVENTORY','37','Merchandise inventory'),
		('INPUT_TAX','44566','Deductible input tax'),('OUTPUT_TAX','44571','Collected output tax'),
		('ACCOUNTS_PAYABLE','401','Trade payables'),('ACCOUNTS_RECEIVABLE','411','Trade receivables'),
		('SALES','707','Merchandise sales'),('COGS','6037','Cost of goods sold'),
		('EQUITY','101','Opening equity')) AS accounts(role,code,name)`, org)
	if err != nil {
		return err
	}
	today := time.Now().UTC()
	// The bundled demo invoice is dated 2026; keep its period open in later years.
	_, err = conn.Exec(ctx, `INSERT INTO accounting.fiscal_periods(organization_id,year)
		VALUES($1,2026),($1,$2) ON CONFLICT DO NOTHING`, org, today.Year())
	if err != nil {
		return err
	}
	entryID := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO accounting.journal_entries(organization_id,id,source_type,source_id,reference,entry_date,currency)
		VALUES($1,$2,'OPENING',$3,'Opening balance',$4,'USD')`, org, entryID, sourceID, today)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `INSERT INTO accounting.journal_lines(organization_id,id,entry_id,role,debit,credit) VALUES
		($1,$2,$3,'CASH',5000,0),($1,$4,$3,'EQUITY',0,5000)`, org, uuid.NewString(), entryID, uuid.NewString())
	return err
}

func (r Repository) Post(ctx context.Context, p Posting) error {
	if err := p.Entry.Validate(); err != nil {
		return err
	}
	if p.SourceType != "INVOICE" && p.SourceType != "SETTLEMENT" {
		return errors.New("invalid accounting source type")
	}
	conn := r.DB.Conn(ctx)
	var open bool
	err := conn.QueryRow(ctx, `SELECT NOT closed FROM accounting.fiscal_periods
 WHERE organization_id=$1 AND year=EXTRACT(YEAR FROM $2::date) FOR SHARE`, p.OrganizationID, p.Date).Scan(&open)
	if err != nil || !open {
		return errors.New("La période comptable est fermée ou indisponible.")
	}
	id := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO accounting.journal_entries(organization_id,id,source_type,source_id,reference,entry_date,currency) VALUES($1,$2,$3,$4,$5,$6,$7)`, p.OrganizationID, id, p.SourceType, p.SourceID, p.Reference, p.Date, p.Currency)
	if err != nil {
		return err
	}
	for _, line := range p.Entry.Lines {
		_, err = conn.Exec(ctx, `INSERT INTO accounting.journal_lines(organization_id,id,entry_id,role,debit,credit) VALUES($1,$2,$3,$4,$5,$6)`, p.OrganizationID, uuid.NewString(), id, line.Role, line.Debit.String(), line.Credit.String())
		if err != nil {
			return err
		}
	}
	return nil
}

func (r Repository) Journal(ctx context.Context, org, code string) ([]JournalRow, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT e.reference,e.entry_date::text,e.source_id::text,a.code,a.name,l.debit::text,l.credit::text FROM accounting.journal_entries e JOIN accounting.journal_lines l ON l.organization_id=e.organization_id AND l.entry_id=e.id JOIN accounting.accounts a ON a.organization_id=l.organization_id AND a.role=l.role WHERE e.organization_id=$1 AND ($2='' OR a.code=$2) ORDER BY e.created_at DESC,a.code LIMIT 400`, org, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []JournalRow{}
	for rows.Next() {
		row := JournalRow{}
		if err := rows.Scan(&row.Reference, &row.Date, &row.SourceID, &row.Code, &row.Name, &row.Debit, &row.Credit); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (r Repository) TrialBalance(ctx context.Context, org string) ([]BalanceRow, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT a.code,a.name,
		coalesce(sum(l.debit),0)::text,coalesce(sum(l.credit),0)::text,
		greatest(coalesce(sum(l.debit-l.credit),0),0)::text,
		greatest(coalesce(sum(l.credit-l.debit),0),0)::text
		FROM accounting.accounts a
		LEFT JOIN accounting.journal_lines l ON l.organization_id=a.organization_id AND l.role=a.role
		WHERE a.organization_id=$1 GROUP BY a.code,a.name ORDER BY a.code`, org)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []BalanceRow{}
	for rows.Next() {
		row := BalanceRow{}
		if err := rows.Scan(&row.Code, &row.Name, &row.Debit, &row.Credit, &row.DebitBalance, &row.CreditBalance); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
