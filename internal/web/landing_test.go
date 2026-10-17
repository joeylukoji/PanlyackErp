package web

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLandingIsPublicAndLinksToExistingAuthenticationPages(t *testing.T) {
	server := Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handler := server.Router()
	for _, path := range []string{"/", "/login", "/register"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost"+path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d", path, response.Code)
		}
		if path != "/" {
			continue
		}
		for _, expected := range []string{`href="/login"`, `href="/register"`, `id="features"`, `id="workflow"`, `id="questions"`, `Aperçu illustratif`} {
			if !strings.Contains(response.Body.String(), expected) {
				t.Errorf("landing missing %s", expected)
			}
		}
	}
	for _, path := range []string{"landing.css", "manrope.ttf", "panlyack-logo.png", "dashboard-growth.jpg", "nav-icons.svg", "sidebar.js"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost/static/"+path, nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Errorf("landing asset %s unavailable: %d", path, response.Code)
		}
	}
}
