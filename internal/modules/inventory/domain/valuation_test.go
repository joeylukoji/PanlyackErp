package domain

import (
	"github.com/shopspring/decimal"
	"testing"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestValuation(t *testing.T) {
	lots := []Lot{{Quantity: d("10"), Cost: d("2")}, {Quantity: d("10"), Cost: d("4")}}
	for _, tc := range []struct{ method, want string }{{"FIFO", "28"}, {"AVCO", "36"}} {
		cost, _, err := Allocate(lots, d("12"), tc.method)
		if err != nil || !cost.Equal(d(tc.want)) {
			t.Fatalf("%s: %s %v", tc.method, cost, err)
		}
	}
	if _, _, err := Allocate(lots, d("21"), "FIFO"); err == nil {
		t.Fatal("oversold stock")
	}
	if _, _, err := Allocate(lots, d("-1"), "FIFO"); err == nil {
		t.Fatal("negative issue")
	}
}
