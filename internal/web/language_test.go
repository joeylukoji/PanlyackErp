package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLanguageSwitchAndSafeReturn(t *testing.T) {
	s := Server{}
	for _, destination := range []string{"/app/accounting?account=512", "https://example.com", "//example.com", "/%2fexample.com", "/%5cexample.com", "/%0aLocation:x", "/\\example.com", "/\nLocation:x", ""} {
		r := httptest.NewRequest("GET", "/language?lang=en&return_to="+url.QueryEscape(destination), nil)
		w := httptest.NewRecorder()
		s.switchLanguage(w, r)
		want := "/"
		if destination == "/app/accounting?account=512" {
			want = destination
		}
		if w.Code != 303 || w.Header().Get("Location") != want {
			t.Errorf("unsafe return %q: status=%d location=%q", destination, w.Code, w.Header().Get("Location"))
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != "panlyack_language" || cookies[0].Value != "en" || !cookies[0].HttpOnly || cookies[0].Path != "/" || cookies[0].SameSite != http.SameSiteLaxMode {
			t.Fatal("missing persistent language cookie")
		}
	}
	w := httptest.NewRecorder()
	s.switchLanguage(w, httptest.NewRequest("GET", "/language?lang=xx", nil))
	if w.Code != 400 || len(w.Result().Cookies()) != 0 {
		t.Fatal("invalid language accepted")
	}
}

func TestPublicEnglishPagesAndPricing(t *testing.T) {
	s := Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handler := s.Router()
	for _, tc := range []struct{ path, text string }{
		{"/", "Up to 200 invoices per month"}, {"/login", "Sign in"}, {"/register", "Create an account"},
	} {
		r := httptest.NewRequest("GET", "http://localhost"+tc.path, nil)
		r.AddCookie(&http.Cookie{Name: "panlyack_language", Value: "en"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		body := w.Body.String()
		if w.Code != 200 || w.Header().Get("Content-Language") != "en" || !strings.Contains(body, `lang="en"`) || !strings.Contains(body, tc.text) {
			t.Errorf("English page %s unavailable: %d, expected %q", tc.path, w.Code, tc.text)
		}
		if tc.path == "/" {
			for _, text := range []string{"$49", "$79", "$150", "Up to 500 invoices per month", "Unlimited invoices", `id="pricing"`, `href="/register"`} {
				if !strings.Contains(body, text) {
					t.Errorf("English pricing missing %q", text)
				}
			}
		}
	}
	fr := httptest.NewRecorder()
	handler.ServeHTTP(fr, httptest.NewRequest("GET", "http://localhost/", nil))
	for _, text := range []string{"49 $", "79 $", "150 $", "Jusqu’à 200 factures par mois", "Jusqu’à 500 factures par mois", "Factures illimitées"} {
		if !strings.Contains(fr.Body.String(), text) {
			t.Errorf("French pricing missing %q", text)
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/login", strings.NewReader("csrf=invalid"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: "panlyack_language", Value: "en"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "Session expired") {
		t.Fatal("CSRF errors are not localized")
	}
}
