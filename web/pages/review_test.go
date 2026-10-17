package pages

import (
	"bytes"
	"context"
	"strings"
	"testing"

	documents "panlyackerp/internal/modules/documents/domain"
)

func TestReviewPreservesForeignCurrencyAndExplainsUSPolicyBeforeConfirmation(t *testing.T) {
	invoice := documents.Invoice{ID: "qa", Status: "NEEDS_REVIEW", Direction: "PURCHASE", Currency: "EUR", Total: "1",
		Lines: []documents.Line{{Name: "Paper", Unit: "each", Quantity: "1", UnitPrice: "1", TaxRate: "0"}}}
	for _, standard := range []string{"US_GAAP", "FR_PCG"} {
		var output bytes.Buffer
		if err := Review(invoice, "csrf", "", "QA", nil, "", standard).Render(context.Background(), &output); err != nil {
			t.Fatal(err)
		}
		body := output.String()
		if !strings.Contains(body, `value="EUR" selected`) {
			t.Fatal("foreign invoice silently displayed USD")
		}
		if strings.Contains(body, "Politique US simplifiée") != (standard == "US_GAAP") {
			t.Fatal("wrong accounting policy shown")
		}
	}
}
