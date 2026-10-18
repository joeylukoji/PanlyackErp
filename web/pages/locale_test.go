package pages

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	inventory "panlyackerp/internal/modules/inventory/application"
	reporting "panlyackerp/internal/modules/reporting/ports"
	"panlyackerp/internal/platform/locale"
)

func TestEnglishTemplateCatalogCoverage(t *testing.T) {
	source, err := os.ReadFile("pages.templ")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../internal/platform/locale/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog map[string]string
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`T\(ctx,\s*("(?:\\.|[^"\\])*")`)
	for _, match := range pattern.FindAllStringSubmatch(string(source), -1) {
		key, err := strconv.Unquote(match[1])
		if err != nil {
			t.Fatal(err)
		}
		key = strings.Join(strings.Fields(key), " ")
		if catalog[key] == "" {
			t.Errorf("missing English translation: %s", key)
		}
	}
}

func TestEnglishRenderingPreservesBusinessData(t *testing.T) {
	ctx := locale.WithLanguage(context.Background(), "en", "/app/inventory")
	var output strings.Builder
	if err := Inventory([]inventory.Product{{SKU: "Factures", Name: "Comptabilité", Unit: "pièce"}}, "Trésorerie").Render(ctx, &output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Inventory", "<td>Factures</td>", "<td>Comptabilité</td>", "<td>pièce</td>", "Trésorerie"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("user data or translated UI changed: %s", expected)
		}
	}
	output.Reset()
	if err := Heading("Stock", "Vérifiez le document et les données avant de comptabiliser.").Render(ctx, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "<h1>Stock</h1>") {
		t.Fatal("invoice identifier was translated")
	}
}

func TestTranslatedCopyRemainsVisibleBesideIcons(t *testing.T) {
	ctx := locale.WithLanguage(context.Background(), "en", "/")
	var output strings.Builder
	if err := Landing().Render(ctx, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "<span>Inventory</span>") {
		t.Fatal("translated icon label was consumed as component children")
	}
	output.Reset()
	if err := Dashboard(reporting.Summary{}, nil, "Example").Render(ctx, &output); err != nil {
		t.Fatal(err)
	}
	for _, heading := range []string{"Purchases and expenses", "Outstanding balances", "Recent invoices"} {
		if !strings.Contains(output.String(), heading) {
			t.Errorf("missing card heading %q", heading)
		}
	}
}
