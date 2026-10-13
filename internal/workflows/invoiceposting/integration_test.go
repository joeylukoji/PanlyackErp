package invoiceposting_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	account "panlyackerp/internal/modules/accounting/adapters/postgres"
	cash "panlyackerp/internal/modules/cash/adapters/postgres"
	documents "panlyackerp/internal/modules/documents/adapters/postgres"
	doc "panlyackerp/internal/modules/documents/domain"
	identity "panlyackerp/internal/modules/identity/adapters/postgres"
	inventory "panlyackerp/internal/modules/inventory/adapters/postgres"
	inventoryapp "panlyackerp/internal/modules/inventory/application"
	reporting "panlyackerp/internal/modules/reporting/adapters/postgres"
	"panlyackerp/internal/platform/audit"
	"panlyackerp/internal/platform/db"
	"panlyackerp/internal/workflows/invoiceposting"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestPostgresPostingAtomicAndIdempotent(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for isolated PostgreSQL integration test")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	name := "panlyack_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	migrations, err := filepath.Glob("../../../db/migrations/*.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatal("migrations not found", err)
	}
	for _, file := range migrations {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		up := strings.Split(string(data), "-- +goose Down")[0]
		if _, err := pool.Exec(ctx, up); err != nil {
			t.Fatalf("migration %s: %v", file, err)
		}
	}
	store := db.Store{Pool: pool}
	docs := documents.Repository{DB: store}
	inv := inventory.Repository{DB: store}
	reports := reporting.Repository{DB: store}
	service := invoiceposting.Service{UOW: store, Documents: docs, Identity: identity.Repository{DB: store}, Inventory: inventoryapp.Service{Repository: inv}, Accounting: account.Repository{DB: store}, Cash: cash.Repository{DB: store}, Reporting: reports, Audit: audit.Log{DB: store}}
	org := "11111111-1111-4111-8111-111111111111"
	invoice := doc.Invoice{ID: uuid.NewString(), OrganizationID: org, DocumentID: uuid.NewString(), Status: "CONFIRMED", Version: 1, Number: "TEST-53740", Partner: "Supplier", IssueDate: "2026-10-13", Currency: "USD", Direction: "PURCHASE", Total: "537.40", Lines: []doc.Line{}, Payments: []doc.Payment{{Method: "CASH", Amount: "537.40"}}}
	for n := range 14 {
		price := "3.8"
		if n == 13 {
			price = "4.34"
		}
		invoice.Lines = append(invoice.Lines, doc.Line{SKU: fmt.Sprintf("TEST-%d", n), Name: fmt.Sprintf("Product %d", n), Unit: "each", Quantity: "10", UnitPrice: price, TaxRate: "0"})
	}
	if err := docs.Create(ctx, invoice, documents.File{Key: uuid.NewString(), MIME: "image/png", Hash: "test"}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- service.Post(ctx, org, invoice.ID, "test") }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	summary, err := reports.Summary(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Cash != "4462.6000" || summary.Inventory != "2537.4000" || summary.Count != 1 {
		t.Fatalf("wrong summary: %+v", summary)
	}
	posted, err := docs.Get(ctx, org, invoice.ID, false)
	if err != nil || posted.Status != "POSTED" {
		t.Fatal(posted, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounting.journal_entries WHERE organization_id=$1 AND source_id=$2`, org, invoice.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate journal", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM platform.audit_log WHERE entity_id=$1 AND action='InvoicePosted'`, invoice.ID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate audit", count, err)
	}
	var balance string
	if err := pool.QueryRow(ctx, `SELECT sum(debit-credit)::text FROM accounting.journal_lines WHERE organization_id=$1`, org).Scan(&balance); err != nil || balance != "0.0000" {
		t.Fatal("unbalanced ledger", balance, err)
	}
	if _, err := docs.Get(ctx, uuid.NewString(), invoice.ID, false); err == nil {
		t.Fatal("cross-organization read succeeded")
	}
	if _, err := pool.Exec(ctx, `UPDATE accounting.journal_entries SET reference='tampered' WHERE source_id=$1`, invoice.ID); err == nil {
		t.Fatal("modified posted journal")
	}
	// A late cash failure must roll back already-written inventory and accounting.
	failed := invoice
	failed.ID = uuid.NewString()
	failed.Number = "ROLLBACK"
	failed.Payments = []doc.Payment{{Method: "BANK", Amount: "537.40"}}
	if err := docs.Create(ctx, failed, documents.File{Key: uuid.NewString(), MIME: "image/png", Hash: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Post(ctx, org, failed.ID, "test"); err == nil {
		t.Fatal("expected insufficient bank funds")
	}
	after, err := reports.Summary(ctx, org)
	if err != nil || after != summary {
		t.Fatal("partial report mutation", after, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM inventory.stock_movements WHERE source_id=$1`, failed.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial stock mutation", count, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounting.journal_entries WHERE source_id=$1`, failed.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial accounting mutation", count, err)
	}
	sale := invoice
	sale.ID = uuid.NewString()
	sale.Number = "CREDIT-SALE"
	sale.Direction = "SALE"
	sale.Total = "20"
	sale.Payments = []doc.Payment{}
	sale.Lines = []doc.Line{{SKU: "TEST-0", Name: "Product 0", Unit: "each", Quantity: "2", UnitPrice: "10", TaxRate: "0"}}
	if err := docs.Create(ctx, sale, documents.File{Key: uuid.NewString(), MIME: "image/png", Hash: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Post(ctx, org, sale.ID, "test"); err != nil {
		t.Fatal(err)
	}
	salesSummary, err := reports.Summary(ctx, org)
	if err != nil || salesSummary.Revenue != "20.0000" || salesSummary.Cost != "7.6000" || salesSummary.Receivables != "20.0000" || salesSummary.Inventory != "2529.8000" {
		t.Fatal("incorrect credit sale projection", salesSummary, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE accounting.fiscal_periods SET closed=true WHERE organization_id=$1`, org); err != nil {
		t.Fatal(err)
	}
	closed := invoice
	closed.ID = uuid.NewString()
	if err := docs.Create(ctx, closed, documents.File{Key: uuid.NewString(), MIME: "image/png", Hash: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Post(ctx, org, closed.ID, "test"); err == nil {
		t.Fatal("posted in closed period")
	}
}
