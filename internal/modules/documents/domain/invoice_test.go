package domain

import "testing"

func TestReviewRequiredAndTotalExact(t *testing.T) {
	i := Invoice{Status: "NEEDS_REVIEW", Direction: "PURCHASE", Number: "INV-1", Partner: "Supplier", IssueDate: "2026-10-13", Currency: "USD", Total: "0.3", Lines: []Line{{Name: "Item", Unit: "each", Quantity: "3", UnitPrice: "0.1", TaxRate: "0"}}}
	if err := i.Transition("POSTED"); err == nil {
		t.Fatal("unconfirmed posting")
	}
	if err := i.Transition("CONFIRMED"); err != nil {
		t.Fatal(err)
	}
	if err := i.Transition("POSTED"); err != nil {
		t.Fatal(err)
	}
	if err := i.Transition("NEEDS_REVIEW"); err == nil {
		t.Fatal("posted invoice changed")
	}
	i.Total = "0.31"
	if _, err := i.Validate(); err == nil {
		t.Fatal("mismatched total")
	}
}
