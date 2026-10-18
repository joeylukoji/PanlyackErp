package web

import (
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"panlyackerp/internal/platform/locale"
)

func (s *Server) language(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		language := "fr"
		if cookie, err := r.Cookie("panlyack_language"); err == nil && cookie.Value == "en" {
			language = "en"
		}
		w.Header().Set("Content-Language", language)
		next.ServeHTTP(w, r.WithContext(locale.WithLanguage(r.Context(), language, r.URL.RequestURI())))
	})
}

func (s *Server) switchLanguage(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("lang")
	if language != "fr" && language != "en" {
		http.Error(w, "Unsupported language", http.StatusBadRequest)
		return
	}
	returnTo := r.URL.Query().Get("return_to")
	parsed, err := url.Parse(returnTo)
	if err != nil || parsed.IsAbs() || parsed.Host != "" ||
		!strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") ||
		strings.Contains(parsed.Path, "\\") || strings.Contains(returnTo, "\\") ||
		strings.IndexFunc(parsed.Path, unicode.IsControl) >= 0 || strings.IndexFunc(returnTo, unicode.IsControl) >= 0 {
		returnTo = "/"
	}
	http.SetCookie(w, &http.Cookie{Name: "panlyack_language", Value: language, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 60 * 60})
	w.Header().Set("Content-Language", language)
	http.Redirect(w, r, returnTo, http.StatusSeeOther)
}

func uiText(r *http.Request, source string) string { return locale.Text(r.Context(), source) }

func (s *Server) userError(w http.ResponseWriter, r *http.Request, message string, status int) {
	http.Error(w, uiText(r, message), status)
}
