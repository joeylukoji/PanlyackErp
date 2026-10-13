package web

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"github.com/go-chi/chi/v5/middleware"
	"net"
	"net/http"
	"strings"
	"time"
)

type csrfKey struct{}

func csrf(r *http.Request) string { token, _ := r.Context().Value(csrfKey{}).(string); return token }
func (s *Server) sign(token string) string {
	mac := hmac.New(sha256.New, []byte(s.Config.Secret))
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}
func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			http.Error(w, "Local demo only", 403)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; frame-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		token := ""
		cookie, err := r.Cookie("panlyack_csrf")
		if err == nil {
			parts := strings.Split(cookie.Value, ".")
			if len(parts) == 2 && hmac.Equal([]byte(parts[1]), []byte(s.sign(parts[0]))) {
				token = parts[0]
			}
		}
		if token == "" {
			data := make([]byte, 32)
			if _, err := rand.Read(data); err != nil {
				http.Error(w, "Session unavailable", 500)
				return
			}
			token = hex.EncodeToString(data)
			http.SetCookie(w, &http.Cookie{Name: "panlyack_csrf", Value: token + "." + s.sign(token), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 3600})
		}
		if r.Method == http.MethodPost {
			if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
				http.Error(w, "Cross-site request rejected", 403)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, s.Config.MaxUpload+(1<<20))
			var parseErr error
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				parseErr = r.ParseMultipartForm(1 << 20)
				if r.MultipartForm != nil {
					defer r.MultipartForm.RemoveAll()
				}
			} else {
				parseErr = r.ParseForm()
			}
			if parseErr != nil {
				http.Error(w, "Invalid or oversized request", 400)
				return
			}
			if !hmac.Equal([]byte(r.PostFormValue("csrf")), []byte(token)) {
				http.Error(w, "Session expirée. Rechargez la page et réessayez.", 403)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), csrfKey{}, token)))
	})
}
func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.Logger.Info("http request", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "duration", time.Since(start), "request_id", middleware.GetReqID(r.Context()), "organization_id", DemoOrganization)
	})
}
