package application

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"panlyackerp/internal/modules/documents/domain"
	storage "panlyackerp/internal/modules/documents/ports"
)

type Extractor interface {
	Extract(context.Context, []byte, string) (domain.Invoice, error)
}
type Repository interface {
	Create(context.Context, domain.Invoice, storage.File) error
	Get(context.Context, string, string, bool) (domain.Invoice, error)
	Save(context.Context, domain.Invoice) error
}
type Storage interface {
	Save([]byte, string) (storage.File, error)
	Remove(string) error
}
type UnitOfWork interface {
	WithinTransaction(context.Context, func(context.Context) error) error
}
type Audit interface {
	Append(context.Context, string, string, string, string, any) error
}
type Service struct {
	Repository Repository
	Storage    Storage
	Extractor  Extractor
	UOW        UnitOfWork
	Audit      Audit
}

func (s Service) Upload(ctx context.Context, org, actor string, data []byte, mime, category string) (string, error) {
	if category != "stock" && category != "expense" {
		return "", errors.New("choose merchandise or expense")
	}
	file, err := s.Storage.Save(data, mime)
	if err != nil {
		return "", err
	}
	invoice, err := s.Extractor.Extract(ctx, data, mime)
	if err != nil {
		_ = s.Storage.Remove(file.Key)
		return "", err
	}
	invoice.ID = uuid.NewString()
	invoice.DocumentID = uuid.NewString()
	invoice.OrganizationID = org
	invoice.Status = "NEEDS_REVIEW"
	invoice.Version = 1
	// The operator selects the treatment; an extraction cannot authorize stock or expense effects.
	invoice.Direction = "PURCHASE"
	invoice.PurchaseCategory = category
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.Repository.Create(ctx, invoice, file); err != nil {
			return err
		}
		return s.Audit.Append(ctx, org, actor, "InvoiceUploaded", invoice.ID, map[string]string{"sha256": file.Hash, "mode": invoice.ExtractionMode, "purchase_category": category})
	})
	if err != nil {
		_ = s.Storage.Remove(file.Key)
		return "", err
	}
	return invoice.ID, nil
}

// CreateSale keeps authored data structured and never invokes the extraction adapter.
func (s Service) CreateSale(ctx context.Context, org, actor string, invoice domain.Invoice) (string, error) {
	invoice.ID = uuid.NewString()
	invoice.DocumentID = uuid.NewString()
	invoice.OrganizationID = org
	invoice.Direction = "SALE"
	invoice.PurchaseCategory = ""
	invoice.Status = "NEEDS_REVIEW"
	invoice.ExtractionMode = "structured"
	invoice.Confidence = ""
	invoice.Warnings = []string{}
	if invoice.Payments == nil {
		invoice.Payments = []domain.Payment{}
	}
	invoice.Version = 1
	totals, err := invoice.LineTotals()
	if err != nil {
		return "", err
	}
	invoice.Total = totals.Total.StringFixed(4)
	if _, err := invoice.Validate(); err != nil {
		return "", err
	}
	data, err := json.Marshal(invoice)
	if err != nil {
		return "", err
	}
	file, err := s.Storage.Save(data, "application/json")
	if err != nil {
		return "", err
	}
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.Repository.Create(ctx, invoice, file); err != nil {
			return err
		}
		return s.Audit.Append(ctx, org, actor, "SaleInvoiceCreated", invoice.ID, invoice)
	})
	if err != nil {
		_ = s.Storage.Remove(file.Key)
		return "", err
	}
	return invoice.ID, nil
}

func (s Service) Review(ctx context.Context, org, actor string, edited domain.Invoice, confirm bool) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context) error {
		current, err := s.Repository.Get(ctx, org, edited.ID, true)
		if err != nil {
			return err
		}
		if current.Status != "NEEDS_REVIEW" {
			return errors.New("reopen review before editing a confirmed invoice")
		}
		if current.Version != edited.Version {
			return errors.New("invoice changed; reload before saving")
		}
		edited.OrganizationID = org
		edited.DocumentID = current.DocumentID
		edited.Status = current.Status
		edited.ExtractionMode = current.ExtractionMode
		edited.Confidence = current.Confidence
		edited.Warnings = current.Warnings
		if current.PurchaseCategory != "" && edited.Direction != "PURCHASE" {
			return errors.New("an external purchase or expense must retain purchase direction")
		}
		if current.ExtractionMode == "structured" {
			if edited.Direction != "SALE" || edited.PurchaseCategory == "expense" {
				return errors.New("a generated sales invoice must remain a sale")
			}
			totals, err := edited.LineTotals()
			if err != nil {
				return err
			}
			edited.Total = totals.Total.StringFixed(4)
		}
		if _, err := edited.Validate(); err != nil {
			return err
		}
		action := "InvoiceReviewed"
		if confirm {
			if err := edited.Transition("CONFIRMED"); err != nil {
				return err
			}
			action = "InvoiceConfirmed"
		}
		if err := s.Repository.Save(ctx, edited); err != nil {
			return err
		}
		return s.Audit.Append(ctx, org, actor, action, edited.ID, map[string]any{"before": current, "after": edited})
	})
}

func (s Service) Reopen(ctx context.Context, org, id, actor string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context) error {
		i, err := s.Repository.Get(ctx, org, id, true)
		if err != nil {
			return err
		}
		if err := i.Transition("NEEDS_REVIEW"); err != nil {
			return err
		}
		if err := s.Repository.Save(ctx, i); err != nil {
			return err
		}
		return s.Audit.Append(ctx, org, actor, "InvoiceReopened", id, i)
	})
}
