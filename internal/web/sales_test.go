package web

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	documents "panlyackerp/internal/modules/documents/domain"
	"panlyackerp/internal/platform/config"
)

func TestInvoiceFormUsesPostValuesAndRetainsTrustedMetadata(t *testing.T) {
	data := url.Values{"number": {"S-1"}, "partner": {"Customer"}, "direction": {"SALE"}, "name": {"Item"}, "sku": {"SKU-1"}, "unit": {"each"}, "quantity": {"3"}, "unit_price": {"0.1"}, "tax_rate": {"0"}}
	request := httptest.NewRequest("POST", "http://localhost/app/invoices/sales?partner=Wrong", strings.NewReader(data.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	invoice, err := invoiceForm(request, documents.Invoice{ID: "trusted-id", OrganizationID: "trusted-org", ExtractionMode: "structured"})
	if err != nil || invoice.Partner != "Customer" || invoice.ID != "trusted-id" || invoice.OrganizationID != "trusted-org" || invoice.ExtractionMode != "structured" || invoice.Lines[0].UnitPrice != "0.1" {
		t.Fatal(invoice, err)
	}
	data.Del("quantity")
	request = httptest.NewRequest("POST", "http://localhost/", strings.NewReader(data.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := invoiceForm(request, documents.Invoice{}); err == nil {
		t.Fatal("incomplete line accepted")
	}
}

func TestSaleCreationAndExportsRequireSession(t *testing.T) {
	s := Server{Config: config.Config{Secret: strings.Repeat("x", 32), MaxUpload: 10 << 20}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, path := range []string{"/app/invoices/sales/new", "/app/invoices/37624828-88f1-4543-97ab-991b535161b8/json", "/app/invoices/37624828-88f1-4543-97ab-991b535161b8/document"} {
		response := httptest.NewRecorder()
		s.Router().ServeHTTP(response, httptest.NewRequest("GET", "http://localhost"+path, nil))
		if response.Code != 303 || response.Header().Get("Location") != "/login" {
			t.Fatalf("unprotected route %s: %d", path, response.Code)
		}
	}
}
