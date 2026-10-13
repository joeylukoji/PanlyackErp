package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	identity "panlyackerp/internal/modules/identity/ports"
	"panlyackerp/web/pages"
)

const sessionCookie = "panlyack_session"

type sessionKey struct{}

func actor(r *http.Request) string {
	session, _ := r.Context().Value(sessionKey{}).(identity.Session)
	return session.UserID
}

func (s *Server) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || s.Auth == nil {
			s.unauthenticated(w, r)
			return
		}
		session, err := s.Auth.Session(r.Context(), cookie.Value)
		if errors.Is(err, pgx.ErrNoRows) {
			s.unauthenticated(w, r)
			return
		}
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if session.OrganizationID != DemoOrganization || session.Role != "OWNER" {
			http.Error(w, "Accès refusé", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey{}, session)))
	})
}

func (s *Server) unauthenticated(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
		http.Error(w, "Authentification requise", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, pages.Login(csrf(r), ""))
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil {
		http.Error(w, "Authentication unavailable", 503)
		return
	}
	email, password := r.PostFormValue("email"), r.PostFormValue("password")
	if len(email) > 254 || len(password) > 128 {
		s.render(w, r, pages.Login(csrf(r), "Adresse e-mail ou mot de passe incorrect."))
		return
	}
	session, err := s.Auth.Authenticate(r.Context(), DemoOrganization, email, password)
	if errors.Is(err, identity.ErrInvalidCredentials) {
		s.render(w, r, pages.Login(csrf(r), "Adresse e-mail ou mot de passe incorrect."))
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	token, err := s.Auth.CreateSession(r.Context(), session)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int((8 * time.Hour).Seconds())})
	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (s *Server) logoutPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, pages.Logout(csrf(r)))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil && s.Auth != nil {
		if err := s.Auth.RevokeSession(r.Context(), cookie.Value); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
