package postgres

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"panlyackerp/internal/modules/accounting/ports"
	"panlyackerp/internal/platform/db"
)

type Repository struct{ DB db.Store }
type Posting = ports.Posting
type JournalRow = ports.JournalRow

func (r Repository) Post(ctx context.Context, p Posting) error {
	if err := p.Entry.Validate(); err != nil {
		return err
	}
	conn := r.DB.Conn(ctx)
	var open bool
	err := conn.QueryRow(ctx, `SELECT NOT closed FROM accounting.fiscal_periods
 WHERE organization_id=$1 AND year=EXTRACT(YEAR FROM $2::date) FOR SHARE`, p.OrganizationID, p.Date).Scan(&open)
	if err != nil || !open {
		return errors.New("fiscal period is closed or not configured")
	}
	id := uuid.NewString()
	_, err = conn.Exec(ctx, `INSERT INTO accounting.journal_entries(organization_id,id,source_type,source_id,reference,entry_date,currency) VALUES($1,$2,'INVOICE',$3,$4,$5,$6)`, p.OrganizationID, id, p.SourceID, p.Reference, p.Date, p.Currency)
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

func (r Repository) Journal(ctx context.Context, org string) ([]JournalRow, error) {
	rows, err := r.DB.Conn(ctx).Query(ctx, `SELECT e.reference,e.entry_date::text,e.source_id::text,a.code,a.name,l.debit::text,l.credit::text FROM accounting.journal_entries e JOIN accounting.journal_lines l ON l.organization_id=e.organization_id AND l.entry_id=e.id JOIN accounting.accounts a ON a.organization_id=l.organization_id AND a.role=l.role WHERE e.organization_id=$1 ORDER BY e.created_at DESC,a.code LIMIT 400`, org)
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
