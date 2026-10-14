package domain

import (
	"github.com/shopspring/decimal"
	"testing"
)

func TestPostingPolicies(t *testing.T) {
	for _, engine := range []PostingEngine{FrenchPCG{}, USGAAP{}} {
		for _, direction := range []string{"PURCHASE", "SALE"} {
			for _, paid := range []int64{0, 300, 500} {
				op := Operation{Direction: direction, Net: decimal.NewFromInt(450), Tax: decimal.NewFromInt(50), Total: decimal.NewFromInt(500), Cost: decimal.NewFromInt(100), Payments: []Payment{{Method: "CASH", Amount: decimal.NewFromInt(paid)}}}
				entry, err := engine.Build(op)
				if err != nil {
					t.Fatal(err)
				}
				if err := entry.Reverse().Validate(); err != nil {
					t.Fatal(err)
				}
				op.Payments[0].Amount = decimal.NewFromInt(501)
				if _, err := engine.Build(op); err == nil {
					t.Fatal("accepted overpayment")
				}
			}
		}
	}
	if (Entry{Lines: []Line{{Role: "CASH", Debit: decimal.NewFromInt(1)}, {Role: "SALES", Credit: decimal.NewFromInt(2)}}}).Validate() == nil {
		t.Fatal("accepted unbalanced journal")
	}
}

func TestSettlementEntry(t *testing.T) {
	amount := decimal.NewFromInt(30)
	for _, tc := range []struct {
		direction, debit, credit string
	}{
		{"PURCHASE", "ACCOUNTS_PAYABLE", "CASH"},
		{"SALE", "BANK", "ACCOUNTS_RECEIVABLE"},
	} {
		method := "CASH"
		if tc.direction == "SALE" {
			method = "BANK"
		}
		entry, err := SettlementEntry(tc.direction, method, amount)
		if err != nil || len(entry.Lines) != 2 || entry.Lines[0].Role != tc.debit || entry.Lines[1].Role != tc.credit {
			t.Fatalf("%s settlement: %+v, %v", tc.direction, entry, err)
		}
	}
	for _, tc := range []struct {
		direction, method string
		amount            decimal.Decimal
	}{
		{"PURCHASE", "CASH", decimal.Zero},
		{"SALE", "UNKNOWN", amount},
		{"OTHER", "BANK", amount},
	} {
		if _, err := SettlementEntry(tc.direction, tc.method, tc.amount); err == nil {
			t.Fatalf("accepted invalid settlement: %+v", tc)
		}
	}
}
