package web

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	accounting "panlyackerp/internal/modules/accounting/adapters/postgres"
	cash "panlyackerp/internal/modules/cash/adapters/postgres"
	"panlyackerp/internal/modules/documents/adapters/local"
	docstore "panlyackerp/internal/modules/documents/adapters/postgres"
	docapp "panlyackerp/internal/modules/documents/application"
	documents "panlyackerp/internal/modules/documents/domain"
	identity "panlyackerp/internal/modules/identity/ports"
	inventory "panlyackerp/internal/modules/inventory/adapters/postgres"
	reporting "panlyackerp/internal/modules/reporting/adapters/postgres"
	"panlyackerp/internal/platform/config"
	"panlyackerp/internal/workflows/invoiceposting"
	"panlyackerp/internal/workflows/registration"
	"panlyackerp/internal/workflows/settlement"
	assets "panlyackerp/web"
	"panlyackerp/web/pages"
	"strconv"
	"time"
)

const DemoOrganization = "11111111-1111-4111-8111-111111111111"

type Server struct {
	Config          config.Config
	Logger          *slog.Logger
	Pool            *pgxpool.Pool
	Documents       docstore.Repository
	DocumentService docapp.Service
	Storage         local.Storage
	Posting         invoiceposting.Service
	Inventory       inventory.Repository
	Accounting      accounting.Repository
	Cash            cash.Repository
	Reporting       reporting.Repository
	Auth            identity.Auth
	Registration    registration.Service
	Settlement      settlement.Service
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, s.security, s.logRequest)
	r.Get("/health/live", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := s.Pool.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Write([]byte("ready"))
	})
	static, _ := fs.Sub(assets.Assets, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	r.Get("/login", s.loginPage)
	r.Post("/login", s.login)
	r.Get("/register", s.registerPage)
	r.Post("/register", s.register)
	r.Post("/logout", s.logout)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		s.render(w, r, pages.Landing())
	})
	r.Group(func(r chi.Router) {
		r.Use(s.requireSession)
		r.Get("/app/dashboard", s.dashboard)
		r.Get("/logout", s.logoutPage)
		r.Get("/app/_partials/dashboard/kpis", s.kpis)
		r.Get("/app/invoices", s.invoices)
		r.Get("/app/invoices/new", func(w http.ResponseWriter, r *http.Request) {
			s.render(w, r, pages.Upload(csrf(r), "", organizationName(r), "stock"))
		})
		r.Post("/app/invoices/upload", s.upload)
		r.Get("/app/invoices/sales/new", s.newSale)
		r.Post("/app/invoices/sales", s.createSale)
		r.Get("/app/invoices/{id}/json", s.invoiceJSON)
		r.Get("/app/invoices/{id}", s.review)
		r.Get("/app/invoices/{id}/document", s.document)
		r.Post("/app/invoices/{id}/review", s.saveReview)
		r.Post("/app/invoices/{id}/post", s.post)
		r.Get("/app/invoices/{id}/accounting-proposal", s.accountingProposal)
		r.Post("/app/invoices/{id}/settlements", s.settle)
		r.Post("/app/invoices/{id}/reopen", s.reopen)
		r.Get("/app/inventory", s.inventory)
		r.Get("/app/accounting", s.accounting)
		r.Get("/app/cash", s.cash)
		r.Get("/api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
			data, err := s.Reporting.Summary(r.Context(), organizationID(r))
			if err != nil {
				s.fail(w, r, err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(data)
		})
	})
	return r
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		s.Logger.Error("render failed", "error", err, "request_id", middleware.GetReqID(r.Context()))
	}
}
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.Logger.Error("request failed", "error", err, "request_id", middleware.GetReqID(r.Context()))
	code, message := http.StatusInternalServerError, "Opération impossible. Réessayez ou consultez les journaux du serveur."
	if errors.Is(err, pgx.ErrNoRows) {
		code = 404
		message = "Document ou produit introuvable."
	}
	http.Error(w, message, code)
}
func safeMessage(err error) string {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) {
		return "Opération refusée par la base de données. Aucune écriture partielle n’a été conservée."
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "Produit introuvable. Vérifiez les SKU avant de comptabiliser une vente."
	}
	return err.Error()
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	summary, err := s.Reporting.Summary(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	list, err := s.Documents.List(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Dashboard(summary, list, organizationName(r)))
}
func (s *Server) kpis(w http.ResponseWriter, r *http.Request) {
	summary, err := s.Reporting.Summary(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.KPI(summary))
}
func (s *Server) invoices(w http.ResponseWriter, r *http.Request) {
	list, err := s.Documents.List(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Invoices(list, organizationName(r)))
}
func (s *Server) inventory(w http.ResponseWriter, r *http.Request) {
	list, err := s.Inventory.List(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Inventory(list, organizationName(r)))
}
func (s *Server) accounting(w http.ResponseWriter, r *http.Request) {
	balances, err := s.Accounting.TrialBalance(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	code := r.URL.Query().Get("account")
	if code != "" {
		found := false
		for _, balance := range balances {
			if balance.Code == code {
				found = true
				break
			}
		}
		if !found {
			http.Error(w, "Unknown account", http.StatusBadRequest)
			return
		}
	}
	list, err := s.Accounting.Journal(r.Context(), organizationID(r), code)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Accounting(list, balances, code, organizationName(r)))
}
func (s *Server) cash(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.Cash.Accounts(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	movements, err := s.Cash.Movements(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Cash(accounts, movements, organizationName(r)))
}

func invoiceID(r *http.Request) (string, error) {
	id := chi.URLParam(r, "id")
	_, err := uuid.Parse(id)
	return id, err
}
func (s *Server) review(w http.ResponseWriter, r *http.Request) {
	id, err := invoiceID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	i, err := s.Documents.Get(r.Context(), organizationID(r), id, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderReview(w, r, i, "")
}

func (s *Server) renderReview(w http.ResponseWriter, r *http.Request, invoice documents.Invoice, message string) {
	payments := []cash.Settlement{}
	due := ""
	if invoice.Status == "POSTED" {
		var err error
		payments, err = s.Cash.Settlements(r.Context(), organizationID(r), invoice.ID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		balance, err := settlement.Outstanding(invoice, payments)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		due = balance.String()
	}
	organization, err := s.Posting.Identity.Get(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Review(invoice, csrf(r), message, organizationName(r), payments, due, organization.AccountingStandard))
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("document")
	if err != nil {
		file, _, err = r.FormFile("camera")
	}
	if r.MultipartForm != nil && len(r.MultipartForm.File["document"])+len(r.MultipartForm.File["camera"]) > 1 {
		if file != nil {
			file.Close()
		}
		s.render(w, r, pages.Upload(csrf(r), "Choisissez un seul document : fichier ou photo.", organizationName(r), r.PostFormValue("purchase_category")))
		return
	}
	if err != nil {
		s.render(w, r, pages.Upload(csrf(r), "Choisissez un fichier PNG, JPEG ou PDF.", organizationName(r), r.PostFormValue("purchase_category")))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.Config.MaxUpload+1))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(data) == 0 || int64(len(data)) > s.Config.MaxUpload {
		s.render(w, r, pages.Upload(csrf(r), "Fichier vide ou trop volumineux (10 Mo maximum).", organizationName(r), r.PostFormValue("purchase_category")))
		return
	}
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "application/pdf" {
		s.render(w, r, pages.Upload(csrf(r), "Format non autorisé. Utilisez PNG, JPEG ou PDF.", organizationName(r), r.PostFormValue("purchase_category")))
		return
	}
	id, err := s.DocumentService.Upload(r.Context(), organizationID(r), actor(r), data, mime, r.PostFormValue("purchase_category"))
	if err != nil {
		s.Logger.Error("upload failed", "error", err)
		s.render(w, r, pages.Upload(csrf(r), "L’extraction a échoué. Vérifiez que le service IA est disponible, puis réessayez.", organizationName(r), r.PostFormValue("purchase_category")))
		return
	}
	s.redirect(w, r, pages.InvoiceURL(id))
}

func (s *Server) saveReview(w http.ResponseWriter, r *http.Request) {
	id, err := invoiceID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	current, err := s.Documents.Get(r.Context(), organizationID(r), id, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	edited, err := invoiceForm(r, current)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	edited.Version, err = strconv.Atoi(r.PostFormValue("version"))
	if err != nil {
		http.Error(w, "Invalid version", http.StatusBadRequest)
		return
	}
	if current.Status == "NEEDS_REVIEW" {
		if r.FormValue("action") == "add_line" && len(edited.Lines) < 100 {
			edited.Lines = append(edited.Lines, documents.Line{Unit: "each", Quantity: "1", UnitPrice: "0", TaxRate: "0"})
			s.renderReview(w, r, edited, "")
			return
		}
		if index, err := strconv.Atoi(r.FormValue("remove_line")); err == nil && index >= 0 && index < len(edited.Lines) {
			edited.Lines = append(edited.Lines[:index], edited.Lines[index+1:]...)
			s.renderReview(w, r, edited, "")
			return
		}
	}
	err = s.DocumentService.Review(r.Context(), organizationID(r), actor(r), edited, r.FormValue("action") == "confirm")
	if err != nil {
		s.renderReview(w, r, edited, safeMessage(err))
		return
	}
	s.redirect(w, r, pages.InvoiceURL(id))
}

func (s *Server) post(w http.ResponseWriter, r *http.Request) {
	id, err := invoiceID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.Posting.Post(r.Context(), organizationID(r), id, actor(r)); err != nil {
		i, loadErr := s.Documents.Get(r.Context(), organizationID(r), id, false)
		if loadErr != nil {
			s.fail(w, r, loadErr)
			return
		}
		s.Logger.Warn("posting rejected", "error", err, "invoice_id", id)
		s.renderReview(w, r, i, safeMessage(err))
		return
	}
	s.redirect(w, r, pages.InvoiceURL(id))
}
func (s *Server) settle(w http.ResponseWriter, r *http.Request) {
	id, err := invoiceID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	command := settlement.Command{
		OrganizationID: organizationID(r),
		InvoiceID:      id,
		ID:             r.FormValue("request_id"),
		Method:         r.FormValue("method"),
		Amount:         r.FormValue("amount"),
		ActorID:        actor(r),
	}
	if err := s.Settlement.Record(r.Context(), command); err != nil {
		invoice, loadErr := s.Documents.Get(r.Context(), organizationID(r), id, false)
		if loadErr != nil {
			s.fail(w, r, loadErr)
			return
		}
		s.Logger.Warn("settlement rejected", "error", err, "invoice_id", id)
		s.renderReview(w, r, invoice, safeMessage(err))
		return
	}
	s.redirect(w, r, pages.InvoiceURL(id))
}
func (s *Server) reopen(w http.ResponseWriter, r *http.Request) {
	id, err := invoiceID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := s.DocumentService.Reopen(r.Context(), organizationID(r), id, actor(r)); err != nil {
		s.fail(w, r, err)
		return
	}
	s.redirect(w, r, pages.InvoiceURL(id))
}
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		w.WriteHeader(200)
		return
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}
func (s *Server) document(w http.ResponseWriter, r *http.Request) {
	id, err := invoiceID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	invoice, err := s.Documents.Get(r.Context(), organizationID(r), id, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if invoice.ExtractionMode == "structured" {
		s.render(w, r, pages.SalesDocument(invoice, organizationName(r)))
		return
	}
	file, err := s.Documents.File(r.Context(), organizationID(r), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if (file.MIME == "image/png" || file.MIME == "image/jpeg") && r.URL.Query().Get("raw") != "1" {
		s.render(w, r, pages.SourceImage(pages.InvoiceURL(id)+"/document?raw=1"))
		return
	}
	path, err := s.Storage.Path(file.Key)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.MIME)
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, path)
}
