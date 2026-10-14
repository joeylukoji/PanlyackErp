package settlement

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	accounting "panlyackerp/internal/modules/accounting/domain"
	accountingports "panlyackerp/internal/modules/accounting/ports"
	cash "panlyackerp/internal/modules/cash/ports"
	"panlyackerp/internal/modules/documents/domain"
	reporting "panlyackerp/internal/modules/reporting/ports"
	"panlyackerp/internal/platform/value"
)

type UnitOfWork interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}

type Documents interface {
	Get(context.Context, string, string, bool) (domain.Invoice, error)
}

type Cash interface {
	Settlements(context.Context, string, string) ([]cash.Settlement, error)
	RecordSettlement(context.Context, string, string, string, string, cash.Payment) error
}

type Accounting interface {
	Post(context.Context, accountingports.Posting) error
}

type Reporting interface {
	ProjectSettlement(context.Context, reporting.Settled) error
}

type Audit interface {
	Append(context.Context, string, string, string, string, any) error
}

type Service struct {
	UOW        UnitOfWork
	Documents  Documents
	Cash       Cash
	Accounting Accounting
	Reporting  Reporting
	Audit      Audit
}

type Command struct {
	OrganizationID, InvoiceID, ID, Method, Amount, ActorID string
}

func Outstanding(invoice domain.Invoice, payments []cash.Settlement) (decimal.Decimal, error) {
	totals, err := invoice.Validate()
	if err != nil {
		return decimal.Zero, err
	}
	due := totals.Total.Sub(totals.Paid)
	for _, payment := range payments {
		amount, err := decimal.NewFromString(payment.Amount)
		if err != nil || !amount.IsPositive() {
			return decimal.Zero, errors.New("invalid recorded settlement")
		}
		due = due.Sub(amount)
	}
	if due.IsNegative() {
		return decimal.Zero, errors.New("settlements exceed invoice total")
	}
	return due, nil
}

func (s Service) Record(ctx context.Context, command Command) error {
	if _, err := uuid.Parse(command.ID); err != nil {
		return errors.New("Demande de règlement invalide. Rechargez la facture.")
	}
	amount, err := value.Parse(command.Amount, 4)
	if err != nil || !amount.IsPositive() {
		return errors.New("Saisissez un montant positif avec quatre décimales maximum.")
	}
	switch command.Method {
	case "CASH", "BANK", "MOBILE_MONEY", "CARD":
	default:
		return errors.New("Choisissez un moyen de paiement valide.")
	}
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context) error {
		invoice, err := s.Documents.Get(ctx, command.OrganizationID, command.InvoiceID, true)
		if err != nil {
			return err
		}
		if invoice.Status != "POSTED" {
			return errors.New("Comptabilisez la facture avant d’enregistrer un règlement.")
		}
		payments, err := s.Cash.Settlements(ctx, command.OrganizationID, command.InvoiceID)
		if err != nil {
			return err
		}
		for _, payment := range payments {
			if payment.ID == command.ID {
				previous, err := decimal.NewFromString(payment.Amount)
				if err == nil && payment.Method == command.Method && previous.Equal(amount) {
					return nil
				}
				return errors.New("Cette demande a déjà été utilisée avec d’autres données. Rechargez la facture.")
			}
		}
		due, err := Outstanding(invoice, payments)
		if err != nil {
			return err
		}
		if amount.GreaterThan(due) {
			return errors.New("Le montant dépasse le solde restant.")
		}
		entry, err := accounting.SettlementEntry(invoice.Direction, command.Method, amount)
		if err != nil {
			return err
		}
		if err := s.Cash.RecordSettlement(
			ctx,
			command.OrganizationID,
			command.ID,
			command.InvoiceID,
			invoice.Direction,
			cash.Payment{Method: command.Method, Amount: amount},
		); err != nil {
			return err
		}
		if err := s.Accounting.Post(ctx, accountingports.Posting{
			OrganizationID: command.OrganizationID,
			SourceType:     "SETTLEMENT",
			SourceID:       command.ID,
			Reference:      "Payment " + invoice.Number,
			Date:           time.Now().UTC().Format("2006-01-02"),
			Currency:       invoice.Currency,
			Entry:          entry,
		}); err != nil {
			return err
		}
		if err := s.Reporting.ProjectSettlement(ctx, reporting.Settled{
			OrganizationID: command.OrganizationID,
			SourceID:       command.ID,
			Direction:      invoice.Direction,
			Amount:         amount,
		}); err != nil {
			return err
		}
		return s.Audit.Append(
			ctx,
			command.OrganizationID,
			command.ActorID,
			"InvoiceSettled",
			command.InvoiceID,
			command,
		)
	})
}
