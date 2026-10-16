package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"panlyackerp/internal/modules/documents/domain"
	"panlyackerp/internal/modules/documents/ports"
)

type memoryDocuments struct {
	invoice domain.Invoice
	data    []byte
	removed bool
	fail    bool
}

func (m *memoryDocuments) Create(_ context.Context, i domain.Invoice, _ ports.File) error {
	m.invoice = i
	return nil
}
func (m *memoryDocuments) Get(context.Context, string, string, bool) (domain.Invoice, error) {
	return m.invoice, nil
}
func (m *memoryDocuments) Save(_ context.Context, i domain.Invoice) error { m.invoice = i; return nil }

type memoryStorage struct{ data *memoryDocuments }

func (m memoryStorage) Save(data []byte, mime string) (ports.File, error) {
	m.data.data = data
	return ports.File{Key: "test"}, nil
}
func (m memoryStorage) Remove(string) error { m.data.removed = true; return nil }
func (m *memoryDocuments) WithinTransaction(ctx context.Context, f func(context.Context) error) error {
	return f(ctx)
}
func (m *memoryDocuments) Append(context.Context, string, string, string, string, any) error {
	if m.fail {
		return errors.New("audit failed")
	}
	return nil
}

type fixedExtractor struct{}

func (fixedExtractor) Extract(context.Context, []byte, string) (domain.Invoice, error) {
	return domain.Invoice{Direction: "SALE", PurchaseCategory: "stock"}, nil
}

func TestCreateSaleUsesStructuredDataAndExactServerTotal(t *testing.T) {
	m := &memoryDocuments{}
	s := Service{Repository: m, Storage: memoryStorage{m}, UOW: m, Audit: m} // A nil extractor must never be called.
	input := domain.Invoice{ID: "untrusted", OrganizationID: "other", Status: "POSTED", Total: "999", Direction: "PURCHASE", PurchaseCategory: "expense", Number: "S-1", Partner: "Customer", IssueDate: "2026-10-16", Currency: "USD", Lines: []domain.Line{{Name: "Item", Unit: "each", Quantity: "3", UnitPrice: "0.1", TaxRate: "10"}}, Payments: []domain.Payment{}}
	id, err := s.CreateSale(context.Background(), "organization", "operator", input)
	if err != nil {
		t.Fatal(err)
	}
	var stored domain.Invoice
	if err := json.Unmarshal(m.data, &stored); err != nil {
		t.Fatal(err)
	}
	if id == "untrusted" || stored.ID != id || stored.Total != "0.3300" || stored.ExtractionMode != "structured" || stored.Status != "NEEDS_REVIEW" || stored.Direction != "SALE" || stored.OrganizationID != "organization" || stored.PurchaseCategory != "" {
		t.Fatalf("incorrect structured sale: %+v", stored)
	}
	edited := m.invoice
	edited.Direction = "PURCHASE"
	if err := s.Review(context.Background(), "organization", "operator", edited, true); err == nil {
		t.Fatal("generated sale converted to a purchase")
	}
	edited = m.invoice
	edited.Total = "999"
	if err := s.Review(context.Background(), "organization", "operator", edited, true); err != nil {
		t.Fatal(err)
	}
	if m.invoice.Total != "0.3300" || m.invoice.Status != "CONFIRMED" {
		t.Fatal("review trusted a client total")
	}
	m.fail = true
	if _, err := s.CreateSale(context.Background(), "organization", "operator", input); err == nil || !m.removed {
		t.Fatal("failed creation did not clean source storage")
	}
}

func TestExternalTreatmentIsOperatorSelected(t *testing.T) {
	m := &memoryDocuments{}
	s := Service{Repository: m, Storage: memoryStorage{m}, Extractor: fixedExtractor{}, UOW: m, Audit: m}
	if _, err := s.Upload(context.Background(), "organization", "operator", []byte("scan"), "image/png", "expense"); err != nil {
		t.Fatal(err)
	}
	if m.invoice.Direction != "PURCHASE" || m.invoice.PurchaseCategory != "expense" || m.invoice.Status != "NEEDS_REVIEW" {
		t.Fatalf("model overrode treatment: %+v", m.invoice)
	}
	if _, err := s.Upload(context.Background(), "organization", "operator", nil, "image/png", "other"); err == nil {
		t.Fatal("accepted unknown category")
	}
}
