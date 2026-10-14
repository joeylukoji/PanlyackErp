package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net/http"
	accounting "panlyackerp/internal/modules/accounting/adapters/postgres"
	cash "panlyackerp/internal/modules/cash/adapters/postgres"
	"panlyackerp/internal/modules/documents/adapters/ai"
	"panlyackerp/internal/modules/documents/adapters/local"
	documents "panlyackerp/internal/modules/documents/adapters/postgres"
	docapp "panlyackerp/internal/modules/documents/application"
	identity "panlyackerp/internal/modules/identity/adapters/postgres"
	inventory "panlyackerp/internal/modules/inventory/adapters/postgres"
	inventoryapp "panlyackerp/internal/modules/inventory/application"
	reporting "panlyackerp/internal/modules/reporting/adapters/postgres"
	"panlyackerp/internal/platform/audit"
	"panlyackerp/internal/platform/config"
	"panlyackerp/internal/platform/db"
	"panlyackerp/internal/web"
	"panlyackerp/internal/workflows/invoiceposting"
	"panlyackerp/internal/workflows/registration"
	"time"
)

func New(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) *web.Server {
	store := db.Store{Pool: pool}
	docs := documents.Repository{DB: store}
	inv := inventory.Repository{DB: store}
	accounts := accounting.Repository{DB: store}
	money := cash.Repository{DB: store}
	reports := reporting.Repository{DB: store}
	log := audit.Log{DB: store}
	auth := identity.AuthRepository{DB: store}
	storage := local.Storage{Root: cfg.StoragePath}
	return &web.Server{
		Config: cfg, Pool: pool, Logger: logger, Auth: auth, Documents: docs, Storage: storage, Inventory: inv, Accounting: accounts, Cash: money, Reporting: reports,
		Registration:    registration.Service{UOW: store, Identity: auth, Cash: money, Accounting: accounts, Reporting: reports},
		DocumentService: docapp.Service{Repository: docs, Storage: storage, Extractor: ai.Client{HTTP: &http.Client{Timeout: 100 * time.Second}, BaseURL: cfg.AIURL, Key: cfg.AIKey}, UOW: store, Audit: log},
		Posting:         invoiceposting.Service{UOW: store, Documents: docs, Identity: identity.Repository{DB: store}, Inventory: inventoryapp.Service{Repository: inv}, Accounting: accounts, Cash: money, Reporting: reports, Audit: log},
	}
}
