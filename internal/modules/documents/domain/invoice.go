package domain

import (
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"panlyackerp/internal/platform/value"
	"strings"
	"time"
)

type Line struct {
	SKU        string `json:"sku"`
	Name       string `json:"name"`
	Unit       string `json:"unit"`
	Quantity   string `json:"quantity"`
	UnitPrice  string `json:"unit_price"`
	TaxRate    string `json:"tax_rate"`
	Confidence string `json:"confidence"`
}

type Payment struct {
	Method string `json:"method"`
	Amount string `json:"amount"`
}

type Invoice struct {
	PurchaseCategory string    `json:"purchase_category,omitempty"`
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organization_id"`
	DocumentID       string    `json:"document_id"`
	Status           string    `json:"status"`
	Direction        string    `json:"direction"`
	Number           string    `json:"number"`
	Partner          string    `json:"partner"`
	IssueDate        string    `json:"issue_date"`
	Currency         string    `json:"currency"`
	Total            string    `json:"total"`
	Lines            []Line    `json:"lines"`
	Payments         []Payment `json:"payments"`
	Confidence       string    `json:"confidence"`
	Warnings         []string  `json:"warnings"`
	ExtractionMode   string    `json:"extraction_mode"`
	Version          int       `json:"version"`
}

type Totals struct{ Net, Tax, Total, Paid decimal.Decimal }

func (i Invoice) Validate() (Totals, error) {
	t := Totals{}
	if i.Direction != "PURCHASE" && i.Direction != "SALE" {
		return t, errors.New("choose purchase or sale")
	}
	if !value.Currency(i.Currency) {
		return t, errors.New("unsupported currency")
	}
	if strings.TrimSpace(i.Number) == "" || len(i.Number) > 100 {
		return t, errors.New("invoice number is required (maximum 100 characters)")
	}
	if strings.TrimSpace(i.Partner) == "" || len(i.Partner) > 200 {
		return t, errors.New("supplier or customer is required (maximum 200 characters)")
	}
	if _, err := time.Parse("2006-01-02", i.IssueDate); err != nil {
		return t, errors.New("invalid invoice date")
	}
	if i.PurchaseCategory != "" && i.PurchaseCategory != "stock" && i.PurchaseCategory != "expense" {
		return t, errors.New("invalid purchase category")
	}
	if i.PurchaseCategory == "expense" && i.Direction != "PURCHASE" {
		return t, errors.New("expenses must use purchase direction")
	}
	t, err := i.LineTotals()
	if err != nil {
		return t, err
	}
	declared, err := value.Parse(i.Total, 4)
	if err != nil || !declared.Equal(t.Total) || !t.Total.IsPositive() {
		return t, fmt.Errorf("invoice total must equal line totals plus tax: %s", t.Total.StringFixed(4))
	}
	if len(i.Payments) > 4 {
		return t, errors.New("at most four payment methods are supported")
	}
	seen := map[string]bool{}
	for _, p := range i.Payments {
		switch p.Method {
		case "CASH", "BANK", "MOBILE_MONEY", "CARD":
		default:
			return t, errors.New("invalid payment method")
		}
		if seen[p.Method] {
			return t, errors.New("combine payments for the same method")
		}
		seen[p.Method] = true
		amount, err := value.Parse(p.Amount, 4)
		if err != nil {
			return t, errors.New("invalid payment amount")
		}
		t.Paid = t.Paid.Add(amount)
	}
	if t.Paid.GreaterThan(t.Total) {
		return t, errors.New("payments exceed invoice total")
	}
	return t, nil
}

// LineTotals computes the same four-decimal totals for authored and extracted invoices.
func (i Invoice) LineTotals() (Totals, error) {
	t := Totals{}
	if len(i.Lines) == 0 || len(i.Lines) > 100 {
		return t, errors.New("invoice must contain 1 to 100 lines")
	}
	for n, line := range i.Lines {
		if strings.TrimSpace(line.Name) == "" || len(line.Name) > 200 || len(line.SKU) > 80 || line.Unit == "" || len(line.Unit) > 20 {
			return t, fmt.Errorf("line %d: name and unit are required", n+1)
		}
		q, err := value.Parse(line.Quantity, 6)
		if err != nil || !q.IsPositive() {
			return t, fmt.Errorf("line %d: invalid quantity", n+1)
		}
		p, err := value.Parse(line.UnitPrice, 4)
		if err != nil {
			return t, fmt.Errorf("line %d: invalid unit price", n+1)
		}
		rate, err := value.Parse(line.TaxRate, 4)
		if err != nil || rate.GreaterThan(decimal.NewFromInt(100)) {
			return t, fmt.Errorf("line %d: invalid tax percentage", n+1)
		}
		net := q.Mul(p).Round(4)
		t.Net = t.Net.Add(net)
		t.Tax = t.Tax.Add(net.Mul(rate).Div(decimal.NewFromInt(100)).Round(4))
	}
	t.Total = t.Net.Add(t.Tax)
	return t, nil
}

func (i *Invoice) Transition(target string) error {
	allowed := (i.Status == "NEEDS_REVIEW" && target == "CONFIRMED") ||
		(i.Status == "CONFIRMED" && target == "NEEDS_REVIEW") || (i.Status == "CONFIRMED" && target == "POSTED")
	if !allowed {
		return fmt.Errorf("cannot change invoice from %s to %s", i.Status, target)
	}
	if target == "CONFIRMED" {
		if _, err := i.Validate(); err != nil {
			return err
		}
	}
	i.Status = target
	return nil
}
