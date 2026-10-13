package web

import (
	"net/http"
	"net/http/httptest"
	"panlyackerp/internal/platform/config"
	"strings"
	"testing"
)

func TestRejectForgedCSRFAndRemoteHost(t *testing.T) {
	s := Server{Config: config.Config{Secret: strings.Repeat("s", 32), MaxUpload: 1024}}
	h := s.security(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		host, method, body string
		want               int
	}{{"attacker.example", "GET", "", 403}, {"localhost", "POST", "csrf=forged", 403}, {"localhost", "GET", "", 204}} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+"/app/invoices", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("got %d want %d", w.Code, tc.want)
		}
	}
}
