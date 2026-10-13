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
	r.Post("/logout", s.logout)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
	})
	r.Group(func(r chi.Router) {
		r.Use(s.requireSession)
		r.Get("/app/dashboard", s.dashboard)
		r.Get("/logout", s.logoutPage)
		r.Get("/app/_partials/dashboard/kpis", s.kpis)
		r.Get("/app/invoices", s.invoices)
		r.Get("/app/invoices/new", func(w http.ResponseWriter, r *http.Request) { s.render(w, r, pages.Upload(csrf(r), "")) })
		r.Post("/app/invoices/upload", s.upload)
		r.Get("/app/invoices/{id}", s.review)
		r.Get("/app/invoices/{id}/document", s.document)
		r.Post("/app/invoices/{id}/review", s.saveReview)
		r.Post("/app/invoices/{id}/post", s.post)
		r.Post("/app/invoices/{id}/reopen", s.reopen)
		r.Get("/app/inventory", s.inventory)
		r.Get("/app/accounting", s.accounting)
		r.Get("/app/cash", s.cash)
		r.Get("/api/v1/dashboard", func(w http.ResponseWriter, r *http.Request) {
			data, err := s.Reporting.Summary(r.Context(), DemoOrganization)
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
	summary, err := s.Reporting.Summary(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	list, err := s.Documents.List(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Dashboard(summary, list))
}
func (s *Server) kpis(w http.ResponseWriter, r *http.Request) {
	summary, err := s.Reporting.Summary(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.KPI(summary))
}
func (s *Server) invoices(w http.ResponseWriter, r *http.Request) {
	list, err := s.Documents.List(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Invoices(list))
}
func (s *Server) inventory(w http.ResponseWriter, r *http.Request) {
	list, err := s.Inventory.List(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Inventory(list))
}
func (s *Server) accounting(w http.ResponseWriter, r *http.Request) {
	list, err := s.Accounting.Journal(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Accounting(list))
}
func (s *Server) cash(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.Cash.Accounts(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	movements, err := s.Cash.Movements(r.Context(), DemoOrganization)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Cash(accounts, movements))
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
	i, err := s.Documents.Get(r.Context(), DemoOrganization, id, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.render(w, r, pages.Review(i, csrf(r), ""))
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("document")
	if err != nil {
		s.render(w, r, pages.Upload(csrf(r), "Choisissez un fichier PNG, JPEG ou PDF."))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, s.Config.MaxUpload+1))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if len(data) == 0 || int64(len(data)) > s.Config.MaxUpload {
		s.render(w, r, pages.Upload(csrf(r), "Fichier vide ou trop volumineux (10 Mo maximum)."))
		return
	}
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "application/pdf" {
		s.render(w, r, pages.Upload(csrf(r), "Format non autorisé. Utilisez PNG, JPEG ou PDF."))
		return
	}
	id, err := s.DocumentService.Upload(r.Context(), DemoOrganization, actor(r), data, mime)
	if err != nil {
		s.Logger.Error("upload failed", "error", err)
		s.render(w, r, pages.Upload(csrf(r), "L’extraction a échoué. Vérifiez que le service IA est disponible, puis réessayez."))
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
	current, err := s.Documents.Get(r.Context(), DemoOrganization, id, false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	edited := current
	edited.Number = r.FormValue("number")
	edited.Partner = r.FormValue("partner")
	edited.IssueDate = r.FormValue("issue_date")
	edited.Direction = r.FormValue("direction")
	edited.Currency = r.FormValue("currency")
	edited.Total = r.FormValue("total")
	edited.Version, err = strconv.Atoi(r.FormValue("version"))
	if err != nil {
		http.Error(w, "Invalid version", 400)
		return
	}
	keys := []string{"sku", "name", "unit", "quantity", "unit_price", "tax_rate"}
	count := len(r.Form["name"])
	if count > 100 {
		http.Error(w, "An invoice supports at most 100 lines", http.StatusBadRequest)
		return
	}
	for _, key := range keys {
		if len(r.Form[key]) != count {
			http.Error(w, "Incomplete invoice lines", 400)
			return
		}
	}
	edited.Lines = make([]documents.Line, count)
	for n := range count {
		edited.Lines[n] = documents.Line{SKU: r.Form["sku"][n], Name: r.Form["name"][n], Unit: r.Form["unit"][n], Quantity: r.Form["quantity"][n], UnitPrice: r.Form["unit_price"][n], TaxRate: r.Form["tax_rate"][n]}
	}
	edited.Payments = []documents.Payment{}
	for _, method := range []string{"CASH", "BANK", "MOBILE_MONEY", "CARD"} {
		amount := r.FormValue("payment_" + method)
		if amount != "" {
			edited.Payments = append(edited.Payments, documents.Payment{Method: method, Amount: amount})
		}
	}
	if current.Status == "NEEDS_REVIEW" {
		if r.FormValue("action") == "add_line" && len(edited.Lines) < 100 {
			edited.Lines = append(edited.Lines, documents.Line{Unit: "each", Quantity: "1", UnitPrice: "0", TaxRate: "0"})
			s.render(w, r, pages.Review(edited, csrf(r), ""))
			return
		}
		if index, err := strconv.Atoi(r.FormValue("remove_line")); err == nil && index >= 0 && index < len(edited.Lines) {
			edited.Lines = append(edited.Lines[:index], edited.Lines[index+1:]...)
			s.render(w, r, pages.Review(edited, csrf(r), ""))
			return
		}
	}
	err = s.DocumentService.Review(r.Context(), DemoOrganization, actor(r), edited, r.FormValue("action") == "confirm")
	if err != nil {
		s.render(w, r, pages.Review(edited, csrf(r), safeMessage(err)))
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
	if err := s.Posting.Post(r.Context(), DemoOrganization, id, actor(r)); err != nil {
		i, loadErr := s.Documents.Get(r.Context(), DemoOrganization, id, false)
		if loadErr != nil {
			s.fail(w, r, loadErr)
			return
		}
		s.Logger.Warn("posting rejected", "error", err, "invoice_id", id)
		s.render(w, r, pages.Review(i, csrf(r), safeMessage(err)))
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
	if err := s.DocumentService.Reopen(r.Context(), DemoOrganization, id, actor(r)); err != nil {
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
	file, err := s.Documents.File(r.Context(), DemoOrganization, id)
	if err != nil {
		s.fail(w, r, err)
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
