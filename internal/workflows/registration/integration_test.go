package registration_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	accounting "panlyackerp/internal/modules/accounting/adapters/postgres"
	cash "panlyackerp/internal/modules/cash/adapters/postgres"
	identity "panlyackerp/internal/modules/identity/adapters/postgres"
	identityports "panlyackerp/internal/modules/identity/ports"
	reporting "panlyackerp/internal/modules/reporting/adapters/postgres"
	"panlyackerp/internal/platform/db"
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
	firstToken, err := service.Register(ctx, "First Workshop", "FIRST@example.com", "a secure passphrase 123")
	if err != nil {
		t.Fatal(err)
	}
	secondToken, err := service.Register(ctx, "Second Workshop", "second@example.com", "another secure passphrase 123")
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
	if _, err := auth.Authenticate(ctx, "first@example.com", "a secure passphrase 123"); err != nil {
		t.Fatalf("registered account cannot log in: %v", err)
	}
	if _, err := auth.Authenticate(ctx, "first@example.com", "wrong password"); !errors.Is(err, identityports.ErrInvalidCredentials) {
		t.Fatalf("wrong password accepted: %v", err)
	}
	if _, err := service.Register(ctx, "Duplicate", "first@example.com", "a secure passphrase 123"); !errors.Is(err, identityports.ErrEmailExists) {
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
		if accountCount != 12 || periodCount == 0 || journalLines != 2 {
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
}
