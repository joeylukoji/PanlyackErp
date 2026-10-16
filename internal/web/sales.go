package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	documents "panlyackerp/internal/modules/documents/domain"
	"panlyackerp/web/pages"
)

func blankSale() documents.Invoice {
	return documents.Invoice{Status: "NEEDS_REVIEW", Direction: "SALE", IssueDate: time.Now().Format("2006-01-02"), Currency: "USD", Payments: []documents.Payment{}, Lines: []documents.Line{{Unit: "each", Quantity: "1", UnitPrice: "0", TaxRate: "0"}}}
}

func (s *Server) newSale(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, pages.NewSale(blankSale(), csrf(r), "", organizationName(r)))
}

// invoiceForm only accepts editable values; identity, state and provenance come from the server.
func invoiceForm(r *http.Request, invoice documents.Invoice) (documents.Invoice, error) {
	invoice.Number = r.PostFormValue("number")
	invoice.Partner = r.PostFormValue("partner")
	invoice.IssueDate = r.PostFormValue("issue_date")
	invoice.Direction = r.PostFormValue("direction")
	invoice.PurchaseCategory = r.PostFormValue("purchase_category")
	invoice.Currency = r.PostFormValue("currency")
	invoice.Total = r.PostFormValue("total")
	keys := []string{"sku", "name", "unit", "quantity", "unit_price", "tax_rate"}
	count := len(r.PostForm["name"])
	if count > 100 {
		return invoice, errors.New("Une facture accepte au maximum 100 lignes.")
	}
	for _, key := range keys {
		if len(r.PostForm[key]) != count {
			return invoice, errors.New("Les lignes de facture sont incomplètes.")
		}
	}
	invoice.Lines = make([]documents.Line, count)
	for n := range count {
		invoice.Lines[n] = documents.Line{SKU: r.PostForm["sku"][n], Name: r.PostForm["name"][n], Unit: r.PostForm["unit"][n], Quantity: r.PostForm["quantity"][n], UnitPrice: r.PostForm["unit_price"][n], TaxRate: r.PostForm["tax_rate"][n]}
	}
	invoice.Payments = []documents.Payment{}
	for _, method := range []string{"CASH", "BANK", "MOBILE_MONEY", "CARD"} {
		if amount := r.PostFormValue("payment_" + method); amount != "" {
			invoice.Payments = append(invoice.Payments, documents.Payment{Method: method, Amount: amount})
		}
	}
	return invoice, nil
}

func (s *Server) createSale(w http.ResponseWriter, r *http.Request) {
	invoice, err := invoiceForm(r, blankSale())
	invoice.Direction = "SALE"
	invoice.PurchaseCategory = ""
	if err != nil {
		s.render(w, r, pages.NewSale(invoice, csrf(r), err.Error(), organizationName(r)))
		return
	}
	if r.PostFormValue("action") == "add_line" && len(invoice.Lines) < 100 {
		invoice.Lines = append(invoice.Lines, documents.Line{Unit: "each", Quantity: "1", UnitPrice: "0", TaxRate: "0"})
		s.render(w, r, pages.NewSale(invoice, csrf(r), "", organizationName(r)))
		return
	}
	if index, err := strconv.Atoi(r.PostFormValue("remove_line")); err == nil && index >= 0 && index < len(invoice.Lines) {
		invoice.Lines = append(invoice.Lines[:index], invoice.Lines[index+1:]...)
		s.render(w, r, pages.NewSale(invoice, csrf(r), "", organizationName(r)))
		return
	}
	id, err := s.DocumentService.CreateSale(r.Context(), organizationID(r), actor(r), invoice)
	if err != nil {
		s.render(w, r, pages.NewSale(invoice, csrf(r), safeMessage(err), organizationName(r)))
		return
	}
	s.redirect(w, r, pages.InvoiceURL(id))
}

func (s *Server) invoiceJSON(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="invoice-`+id+`.json"`)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(invoice); err != nil {
		s.Logger.Error("invoice export failed", "error", err)
	}
}
