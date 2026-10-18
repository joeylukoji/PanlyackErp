package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
	accounting "panlyackerp/internal/modules/accounting/domain"
	"panlyackerp/web/pages"
)

type roleSides struct {
	Debit  []string `json:"debit_roles"`
	Credit []string `json:"credit_roles"`
}

type roleAdvice struct {
	References                []pages.AccountingReference `json:"references"`
	ModelRoles                *roleSides                  `json:"model_roles"`
	PolicyMatch               bool                        `json:"policy_match"`
	PostingAuthorized         bool                        `json:"posting_authorized"`
	HumanConfirmationRequired bool                        `json:"human_confirmation_required"`
}

func sameRoles(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a, b := append([]string{}, left...), append([]string{}, right...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Server) accountingProposal(w http.ResponseWriter, r *http.Request) {
	notice := func(message string) { s.render(w, r, pages.AccountingSuggestion(message, nil, nil)) }
	invoice, err := s.Documents.Get(r.Context(), organizationID(r), chi.URLParam(r, "id"), false)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	organization, err := s.Posting.Identity.Get(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if organization.AccountingStandard != "US_GAAP" || invoice.Status != "CONFIRMED" {
		notice("Le pilote de proposition US concerne les factures confirmées dans un espace US. Les autres factures conservent leur moteur comptable habituel.")
		return
	}
	if invoice.Currency != organization.Currency || organization.Currency != "USD" {
		notice("Ce pilote utilise un espace et une facture en USD. Aucune conversion de devise n’est effectuée.")
		return
	}
	totals, err := invoice.Validate()
	if err != nil {
		notice("Corrigez et confirmez les lignes et montants avant de proposer les comptes.")
		return
	}
	taxTreatment := "NONE"
	if totals.Tax.IsPositive() {
		taxTreatment = "NONRECOVERABLE_PURCHASE_TAX"
	}
	operation := "INVENTORY_PURCHASE"
	if invoice.PurchaseCategory == "expense" {
		operation = "OPERATING_EXPENSE"
	}
	if invoice.Direction == "SALE" {
		operation = "SALE"
		if totals.Tax.IsPositive() {
			taxTreatment = "BUYER_SALES_TAX"
		}
	}
	settlement := "UNPAID"
	if totals.Paid.IsPositive() {
		settlement = "PARTIAL"
	}
	if totals.Paid.Equal(totals.Total) {
		settlement = "PAID"
	}
	methods, products := []string{}, []string{}
	payments := []accounting.Payment{}
	for _, payment := range invoice.Payments {
		amount, _ := decimal.NewFromString(payment.Amount)
		if amount.IsPositive() {
			methods = append(methods, payment.Method)
			payments = append(payments, accounting.Payment{Method: payment.Method, Amount: amount})
		}
	}
	for _, line := range invoice.Lines {
		if len(products) < 12 {
			products = append(products, line.Name)
		}
	}
	body, _ := json.Marshal(map[string]any{"operation": operation, "settlement": settlement, "payment_methods": methods, "products": products, "cost_recognized": false, "tax_treatment": taxTreatment})
	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.Config.AIURL, "/")+"/v1/accounting/propose", bytes.NewReader(body))
	if err != nil {
		notice("Service de proposition indisponible.")
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+s.Config.AIKey)
	response, err := (&http.Client{Timeout: 100 * time.Second}).Do(request)
	if err != nil {
		notice("Service de proposition indisponible. Aucune écriture enregistrée.")
		return
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		notice("Le modèle comptable ne fournit pas encore de proposition exploitable. Aucune écriture enregistrée.")
		return
	}
	proposal := roleAdvice{}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&proposal); err != nil {
		notice("Proposition invalide. Aucune écriture enregistrée.")
		return
	}
	entry, err := (accounting.USGAAP{}).Build(accounting.Operation{Expense: invoice.PurchaseCategory == "expense", Direction: invoice.Direction, Net: totals.Net, Tax: totals.Tax, Total: totals.Total, Payments: payments})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	expected := roleSides{Debit: []string{}, Credit: []string{}}
	for _, line := range entry.Lines {
		if line.Debit.IsPositive() {
			expected.Debit = append(expected.Debit, line.Role)
		}
		if line.Credit.IsPositive() {
			expected.Credit = append(expected.Credit, line.Role)
		}
	}
	if !proposal.PolicyMatch || proposal.PostingAuthorized || !proposal.HumanConfirmationRequired || proposal.ModelRoles == nil ||
		!sameRoles(expected.Debit, proposal.ModelRoles.Debit) || !sameRoles(expected.Credit, proposal.ModelRoles.Credit) {
		notice("La proposition du modèle ne concorde pas avec le moteur de règles. Elle est rejetée ; aucune écriture enregistrée.")
		return
	}
	accounts, err := s.Accounting.Accounts(r.Context(), organizationID(r))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	rows := []pages.SuggestedAccount{}
	for _, line := range entry.Lines {
		found := false
		for _, account := range accounts {
			if account.Role == line.Role {
				side := "Crédit"
				amount := line.Credit
				if line.Debit.IsPositive() {
					side = "Débit"
					amount = line.Debit
				}
				rows = append(rows, pages.SuggestedAccount{Side: side, Code: account.Code, Name: account.Name, Amount: amount.StringFixed(4)})
				found = true
				break
			}
		}
		if !found {
			notice("Un compte du plan de votre entreprise manque. Aucune écriture enregistrée.")
			return
		}
	}
	message := "Proposition expérimentale vérifiée par le moteur de règles. Les comptes et montants ci-dessous proviennent de votre plan et des calculs Go ; rien n’est enregistré avant « Comptabiliser la facture »."
	if invoice.Direction == "SALE" {
		message += " Le coût des ventes et la sortie de stock seront calculés lors de la comptabilisation selon la méthode de votre organisation ; ils ne figurent pas dans cet aperçu."
	}
	s.render(w, r, pages.AccountingSuggestion(message, rows, proposal.References))
}
