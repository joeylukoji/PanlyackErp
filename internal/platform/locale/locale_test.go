package locale

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestRequestLanguageAndWhitespace(t *testing.T) {
	base := context.Background()
	en := WithLanguage(base, "en", "/app/accounting?account=512")
	if got := Text(en, "  Factures\n"); got != "  Invoices\n" {
		t.Fatalf("whitespace changed: %q", got)
	}
	if Text(base, "Factures") != "Factures" || Language(base) != "fr" || Language(WithLanguage(base, "invalid", "/")) != "fr" {
		t.Fatal("language leaked across requests or invalid language was accepted")
	}
	if got := SwitchURL(en, "fr"); got != "/language?lang=fr&return_to=%2Fapp%2Faccounting%3Faccount%3D512" {
		t.Fatalf("return URL changed: %s", got)
	}
}

func TestEnglishCatalogKeepsFormattingArguments(t *testing.T) {
	for source, target := range english {
		if strings.TrimSpace(target) == "" {
			t.Errorf("empty translation: %q", source)
		}
		if strings.Count(source, "%d") != strings.Count(target, "%d") {
			t.Errorf("format arguments changed: %q -> %q", source, target)
		}
	}
	ctx := WithLanguage(context.Background(), "en", "/")
	if got := fmt.Sprintf(Text(ctx, "SKU ligne %d"), 2); got != "SKU on line 2" {
		t.Fatalf("line label: %q", got)
	}
}
