package postgres

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"panlyackerp/internal/modules/reporting/ports"
	"panlyackerp/internal/platform/db"
)

type Repository struct{ DB db.Store }
type Posted = ports.Posted
type Summary = ports.Summary

// Project records the event and its projection in the same transaction.
func (r Repository) Project(ctx context.Context, event Posted) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	tag, err := r.DB.Conn(ctx).Exec(ctx, `INSERT INTO platform.outbox_events(id,organization_id,event_type,source_id,payload,projected_at) VALUES($1,$2,'InvoicePosted',$3,$4,now()) ON CONFLICT(organization_id,event_type,source_id) DO NOTHING`, uuid.NewString(), event.OrganizationID, event.SourceID, payload)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	revenue, purchases, cost, cash, inventory, receivables, payables := decimal.Zero, decimal.Zero, decimal.Zero, event.Paid, event.Cost.Neg(), event.Due, decimal.Zero
	if event.Direction == "PURCHASE" {
		purchases = event.Net
		cash = event.Paid.Neg()
		inventory = event.Net
		receivables = decimal.Zero
		payables = event.Due
	} else {
		revenue = event.Net
		cost = event.Cost
	}
	_, err = r.DB.Conn(ctx).Exec(ctx, `UPDATE reporting.summary SET revenue=revenue+$2,purchases=purchases+$3,cogs=cogs+$4,cash=cash+$5,inventory=inventory+$6,receivables=receivables+$7,payables=payables+$8,posted_count=posted_count+1 WHERE organization_id=$1`, event.OrganizationID, revenue.String(), purchases.String(), cost.String(), cash.String(), inventory.String(), receivables.String(), payables.String())
	return err
}

func (r Repository) Summary(ctx context.Context, org string) (Summary, error) {
	s := Summary{}
	err := r.DB.Conn(ctx).QueryRow(ctx, `SELECT revenue::text,purchases::text,cogs::text,cash::text,inventory::text,receivables::text,payables::text,posted_count FROM reporting.summary WHERE organization_id=$1`, org).Scan(&s.Revenue, &s.Purchases, &s.Cost, &s.Cash, &s.Inventory, &s.Receivables, &s.Payables, &s.Count)
	return s, err
}
