package registration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	accounting "panlyackerp/internal/modules/accounting/adapters/postgres"
	cash "panlyackerp/internal/modules/cash/adapters/postgres"
	documents "panlyackerp/internal/modules/documents/adapters/postgres"
	doc "panlyackerp/internal/modules/documents/domain"
	identity "panlyackerp/internal/modules/identity/adapters/postgres"
	identityports "panlyackerp/internal/modules/identity/ports"
	inventory "panlyackerp/internal/modules/inventory/adapters/postgres"
	inventoryapp "panlyackerp/internal/modules/inventory/application"
	reporting "panlyackerp/internal/modules/reporting/adapters/postgres"
	"panlyackerp/internal/platform/audit"
	"panlyackerp/internal/platform/db"
	"panlyackerp/internal/workflows/invoiceposting"
	"panlyackerp/internal/workflows/registration"
)

func TestRegistrationCreatesIsolatedReadyWorkspaces(t *testing.T) {
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
	name := "panlyack_signup_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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
	auth := identity.AuthRepository{DB: store}
	service := registration.Service{
		UOW: store, Identity: auth,
		Cash:       cash.Repository{DB: store},
		Accounting: accounting.Repository{DB: store},
		Reporting:  reporting.Repository{DB: store},
	}
	firstToken, err := service.Register(ctx, "First Workshop", "FIRST@example.com", "a secure passphrase 123", "FR_PCG")
	if err != nil {
		t.Fatal(err)
	}
	secondToken, err := service.Register(ctx, "Second Workshop", "second@example.com", "another secure passphrase 123", "US_GAAP")
	if err != nil {
		t.Fatal(err)
	}
	first, err := auth.Session(ctx, firstToken)
	if err != nil {
		t.Fatal(err)
	}
	second, err := auth.Session(ctx, secondToken)
	if err != nil {
		t.Fatal(err)
	}
	if first.OrganizationID == second.OrganizationID || first.OrganizationName != "First Workshop" || first.Role != "OWNER" {
		t.Fatalf("unexpected session scope: first=%+v second=%+v", first, second)
	}
	var standard, inventoryCode string
	if err := pool.QueryRow(ctx, `SELECT accounting_standard FROM identity.organizations WHERE id=$1`, second.OrganizationID).Scan(&standard); err != nil || standard != "US_GAAP" {
		t.Fatal("US standard not retained", standard, err)
	}
	if err := pool.QueryRow(ctx, `SELECT code FROM accounting.accounts WHERE organization_id=$1 AND role='INVENTORY'`, second.OrganizationID).Scan(&inventoryCode); err != nil || inventoryCode != "1200" {
		t.Fatal("US chart not seeded", inventoryCode, err)
	}
	if _, err := auth.Authenticate(ctx, "first@example.com", "a secure passphrase 123"); err != nil {
		t.Fatalf("registered account cannot log in: %v", err)
	}
	if _, err := auth.Authenticate(ctx, "first@example.com", "wrong password"); !errors.Is(err, identityports.ErrInvalidCredentials) {
		t.Fatalf("wrong password accepted: %v", err)
	}
	if _, err := service.Register(ctx, "Duplicate", "first@example.com", "a secure passphrase 123", "FR_PCG"); !errors.Is(err, identityports.ErrEmailExists) {
		t.Fatalf("duplicate email error: %v", err)
	}
	if err := auth.SetOwner(ctx, "11111111-1111-4111-8111-111111111111", first.Email, "replaced passphrase 123"); err == nil {
		t.Fatal("bootstrap owner command took over a registered account")
	}
	if _, err := auth.Authenticate(ctx, first.Email, "a secure passphrase 123"); err != nil {
		t.Fatalf("account password changed after rejected owner command: %v", err)
	}
	var duplicates int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM identity.organizations WHERE name='Duplicate'`).Scan(&duplicates); err != nil || duplicates != 0 {
		t.Fatalf("duplicate registration was not rolled back: count=%d err=%v", duplicates, err)
	}
	for _, session := range []identityports.Session{first, second} {
		var accountCount, periodCount, journalLines int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounting.accounts WHERE organization_id=$1`, session.OrganizationID).Scan(&accountCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounting.fiscal_periods WHERE organization_id=$1`, session.OrganizationID).Scan(&periodCount); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM accounting.journal_lines WHERE organization_id=$1`, session.OrganizationID).Scan(&journalLines); err != nil {
			t.Fatal(err)
		}
		if accountCount != 13 || periodCount == 0 || journalLines != 2 {
			t.Fatalf("incomplete accounting setup: accounts=%d periods=%d lines=%d", accountCount, periodCount, journalLines)
		}
		summary, err := (reporting.Repository{DB: store}).Summary(ctx, session.OrganizationID)
		if err != nil || summary.Cash != "5000.0000" || summary.Count != 0 {
			t.Fatalf("incorrect opening summary: %+v %v", summary, err)
		}
		accounts, err := (cash.Repository{DB: store}).Accounts(ctx, session.OrganizationID)
		if err != nil || len(accounts) != 4 {
			t.Fatalf("cash accounts unavailable: %v %v", accounts, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE reporting.summary SET cash=4321 WHERE organization_id=$1`, first.OrganizationID); err != nil {
		t.Fatal(err)
	}
	other, err := (reporting.Repository{DB: store}).Summary(ctx, second.OrganizationID)
	if err != nil || other.Cash != "5000.0000" {
		t.Fatalf("cross-workspace data leak: %+v %v", other, err)
	}
	docs := documents.Repository{DB: store}
	stock := inventory.Repository{DB: store}
	posting := invoiceposting.Service{UOW: store, Documents: docs, Identity: identity.Repository{DB: store},
		Inventory: inventoryapp.Service{Repository: stock}, Accounting: accounting.Repository{DB: store},
		Cash: cash.Repository{DB: store}, Reporting: reporting.Repository{DB: store}, Audit: audit.Log{DB: store}}
	for index, expense := range []bool{false, true} {
		invoice := doc.Invoice{ID: uuid.NewString(), DocumentID: uuid.NewString(), OrganizationID: second.OrganizationID,
			Status: "CONFIRMED", Version: 1, Number: fmt.Sprintf("US-CHECK-%d", index), Partner: "QA supplier",
			IssueDate: "2026-10-17", Currency: "USD", Direction: "PURCHASE", Total: "50",
			Lines:    []doc.Line{{SKU: "QA-RICE", Name: "Rice", Unit: "each", Quantity: "2", UnitPrice: "25", TaxRate: "0"}},
			Payments: []doc.Payment{{Method: "CASH", Amount: "20"}}}
		if expense {
			invoice.PurchaseCategory = "expense"
			invoice.Total = "10"
			invoice.Lines = []doc.Line{{SKU: "QA-DELIVERY", Name: "Delivery service", Unit: "each", Quantity: "1", UnitPrice: "10", TaxRate: "0"}}
			invoice.Payments = []doc.Payment{{Method: "CASH", Amount: "5"}}
		}
		if err := docs.Create(ctx, invoice, documents.File{Key: uuid.NewString(), MIME: "image/png", Hash: "qa-us"}); err != nil {
			t.Fatal(err)
		}
		if err := posting.Post(ctx, first.OrganizationID, invoice.ID, first.UserID); err == nil {
			t.Fatal("cross-organization posting allowed")
		}
		for range 2 {
			if err := posting.Post(ctx, second.OrganizationID, invoice.ID, second.UserID); err != nil {
				t.Fatal(err)
			}
		}
	}
	summary, err := (reporting.Repository{DB: store}).Summary(ctx, second.OrganizationID)
	if err != nil || summary.Cash != "4975.0000" || summary.Inventory != "50.0000" || summary.Payables != "35.0000" || summary.Expenses != "10.0000" || summary.Count != 2 {
		t.Fatalf("incorrect US effects: %+v %v", summary, err)
	}
	products, err := stock.List(ctx, second.OrganizationID)
	if err != nil || len(products) != 1 || products[0].SKU != "QA-RICE" || !products[0].Quantity.Equal(decimal.NewFromInt(2)) {
		t.Fatal("incorrect product quantities or expense created stock", products, err)
	}
	for role, code := range map[string]string{"CASH": "1000", "INVENTORY": "1200", "ACCOUNTS_PAYABLE": "2000", "OPERATING_EXPENSE": "6100"} {
		var actual string
		if err := pool.QueryRow(ctx, `SELECT code FROM accounting.accounts WHERE organization_id=$1 AND role=$2`, second.OrganizationID, role).Scan(&actual); err != nil || actual != code {
			t.Fatal(role, actual, err)
		}
	}
	for index := range 3 {
		invoice := doc.Invoice{ID: uuid.NewString(), DocumentID: uuid.NewString(), OrganizationID: second.OrganizationID,
			Status: "CONFIRMED", Version: 1, Number: fmt.Sprintf("US-TAX-%d", index), Partner: "QA tax counterparty",
			IssueDate: "2026-10-17", Currency: "USD", Direction: "PURCHASE", Total: "54",
			Lines:    []doc.Line{{SKU: "QA-TAX-RICE", Name: "Taxed rice", Unit: "each", Quantity: "2", UnitPrice: "25", TaxRate: "8"}},
			Payments: []doc.Payment{{Method: "CASH", Amount: "20"}}}
		if index == 1 {
			invoice.PurchaseCategory = "expense"
			invoice.Total = "10.8"
			invoice.Lines = []doc.Line{{Name: "Taxed office service", Unit: "each", Quantity: "1", UnitPrice: "10", TaxRate: "8"}}
			invoice.Payments = []doc.Payment{{Method: "CASH", Amount: "5"}}
		}
		if index == 2 {
			invoice.Direction = "SALE"
			invoice.Total = "43.2"
			invoice.Lines[0].Quantity = "1"
			invoice.Lines[0].UnitPrice = "40"
			invoice.Payments = []doc.Payment{{Method: "BANK", Amount: "10"}}
		}
		if err := docs.Create(ctx, invoice, documents.File{Key: uuid.NewString(), MIME: "image/png", Hash: "qa-tax"}); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := posting.Post(ctx, second.OrganizationID, invoice.ID, second.UserID); err != nil {
				t.Fatal(err)
			}
		}
	}
	summary, err = (reporting.Repository{DB: store}).Summary(ctx, second.OrganizationID)
	if err != nil || summary.Cash != "4960.0000" || summary.Inventory != "77.0000" || summary.Payables != "74.8000" || summary.Expenses != "20.8000" || summary.Receivables != "33.2000" || summary.Count != 5 {
		t.Fatalf("US taxed stock/expense/sale effects disagree: %+v %v", summary, err)
	}
	var inputTax, salesTax, inventoryBook string
	if err := pool.QueryRow(ctx, `SELECT coalesce(sum(l.debit-l.credit) FILTER(WHERE l.role='INPUT_TAX'),0)::text,
	 coalesce(sum(l.credit-l.debit) FILTER(WHERE l.role='OUTPUT_TAX'),0)::text,
	 coalesce(sum(l.debit-l.credit) FILTER(WHERE l.role='INVENTORY'),0)::text
	 FROM accounting.journal_lines l
	 WHERE l.organization_id=$1`, second.OrganizationID).Scan(&inputTax, &salesTax, &inventoryBook); err != nil {
		t.Fatal(err)
	}
	if !decimal.RequireFromString(inputTax).IsZero() || !decimal.RequireFromString(salesTax).Equal(decimal.RequireFromString("3.2")) || !decimal.RequireFromString(inventoryBook).Equal(decimal.NewFromInt(77)) {
		t.Fatal("US tax liability or inventory book disagrees", inputTax, salesTax, inventoryBook)
	}
}
