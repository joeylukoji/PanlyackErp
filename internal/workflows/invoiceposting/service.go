package invoiceposting

import (
	"context"
	"errors"
	"github.com/shopspring/decimal"
	account "panlyackerp/internal/modules/accounting/domain"
	accountstore "panlyackerp/internal/modules/accounting/ports"
	cash "panlyackerp/internal/modules/cash/ports"
	documents "panlyackerp/internal/modules/documents/domain"
	identity "panlyackerp/internal/modules/identity/domain"
	inventory "panlyackerp/internal/modules/inventory/application"
	reporting "panlyackerp/internal/modules/reporting/ports"
)

type UnitOfWork interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Documents interface {
	Get(context.Context, string, string, bool) (documents.Invoice, error)
	Save(context.Context, documents.Invoice) error
}
type Identity interface {
	Get(context.Context, string) (identity.Organization, error)
}
type Inventory interface {
	Apply(context.Context, inventory.Command) (decimal.Decimal, error)
}
type Accounting interface {
	Post(context.Context, accountstore.Posting) error
}
type Cash interface {
	Record(context.Context, cash.Command) error
}
type Reporting interface {
	Project(context.Context, reporting.Posted) error
}
type Audit interface {
	Append(context.Context, string, string, string, string, any) error
}
type Service struct {
	UOW        UnitOfWork
	Documents  Documents
	Identity   Identity
	Inventory  Inventory
	Accounting Accounting
	Cash       Cash
	Reporting  Reporting
	Audit      Audit
}

func (s Service) Post(ctx context.Context, org, id, actor string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context) error {
		invoice, err := s.Documents.Get(ctx, org, id, true)
		if err != nil {
			return err
		}
		if invoice.Status == "POSTED" {
			return nil
		}
		if invoice.Status != "CONFIRMED" {
			return errors.New("invoice must be confirmed before posting")
		}
		organization, err := s.Identity.Get(ctx, org)
		if err != nil {
			return err
		}
		if invoice.Currency != organization.Currency {
			return errors.New("invoice currency differs from organization currency")
		}
		totals, err := invoice.Validate()
		if err != nil {
			return err
		}
		cost := decimal.Zero
		if invoice.PurchaseCategory != "expense" {
			items := make([]inventory.Item, 0, len(invoice.Lines))
			for _, line := range invoice.Lines {
				q, _ := decimal.NewFromString(line.Quantity)
				p, _ := decimal.NewFromString(line.UnitPrice)
				items = append(items, inventory.Item{SKU: line.SKU, Name: line.Name, Unit: line.Unit, Quantity: q, Price: p})
			}
			cost, err = s.Inventory.Apply(ctx, inventory.Command{OrganizationID: org, SourceID: id, Direction: invoice.Direction, Method: organization.InventoryMethod, Items: items})
			if err != nil {
				return err
			}
		}
		operation := account.Operation{Expense: invoice.PurchaseCategory == "expense", Direction: invoice.Direction, Net: totals.Net, Tax: totals.Tax, Total: totals.Total, Cost: cost, Payments: []account.Payment{}}
		payments := []cash.Payment{}
		for _, p := range invoice.Payments {
			amount, _ := decimal.NewFromString(p.Amount)
			operation.Payments = append(operation.Payments, account.Payment{Method: p.Method, Amount: amount})
			payments = append(payments, cash.Payment{Method: p.Method, Amount: amount})
		}
		var engine account.PostingEngine = account.FrenchPCG{}
		if organization.AccountingStandard == "US_GAAP" {
			engine = account.USGAAP{}
		}
		entry, err := engine.Build(operation)
		if err != nil {
			return err
		}
		if err := s.Accounting.Post(ctx, accountstore.Posting{OrganizationID: org, SourceType: "INVOICE", SourceID: id, Reference: invoice.Number, Date: invoice.IssueDate, Currency: invoice.Currency, Entry: entry}); err != nil {
			return err
		}
		if err := s.Cash.Record(ctx, cash.Command{OrganizationID: org, SourceID: id, Direction: invoice.Direction, Payments: payments}); err != nil {
			return err
		}
		if err := invoice.Transition("POSTED"); err != nil {
			return err
		}
		if err := s.Documents.Save(ctx, invoice); err != nil {
			return err
		}
		if err := s.Audit.Append(ctx, org, actor, "InvoicePosted", id, invoice); err != nil {
			return err
		}
		return s.Reporting.Project(ctx, reporting.Posted{Expense: invoice.PurchaseCategory == "expense", OrganizationID: org, SourceID: id, Direction: invoice.Direction, Net: totals.Net, Cost: cost, Paid: totals.Paid, Due: totals.Total.Sub(totals.Paid)})
	})
}
