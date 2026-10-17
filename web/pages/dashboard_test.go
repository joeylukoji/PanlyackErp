package pages

import "testing"

func TestSpendingUsesExactDecimalsAndHandlesEmptyOrNegativeTotals(t *testing.T) {
	tests := []struct {
		purchases, expenses, total, share string
		chart                             bool
	}{
		{"0.1", "0.2", "0.30", "33.33", true},
		{"0", "0", "0.00", "0", false},
		{"10", "0", "10.00", "100.00", true},
		{"-10", "20", "10.00", "0", false},
		{"invalid", "20", "—", "0", false},
	}
	for _, test := range tests {
		got := spending(test.purchases, test.expenses)
		if got.Total != test.total || got.Share != test.share || got.Chart != test.chart {
			t.Errorf("spending(%s,%s) = %+v", test.purchases, test.expenses, got)
		}
	}
}
