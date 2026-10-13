package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	identity "panlyackerp/internal/modules/identity/ports"
)

type fakeAuth struct {
	session identity.Session
	err     error
}

func (a fakeAuth) Authenticate(context.Context, string, string, string) (identity.Session, error) {
	return a.session, a.err
}
func (a fakeAuth) CreateSession(context.Context, identity.Session) (string, error) {
	return "token", nil
}
func (a fakeAuth) Session(context.Context, string) (identity.Session, error) { return a.session, a.err }
func (a fakeAuth) RevokeSession(context.Context, string) error               { return nil }

func TestPrivateRouteRequiresOrganizationMembership(t *testing.T) {
	for _, tc := range []struct {
		name   string
		auth   fakeAuth
		cookie bool
		want   int
	}{
		{name: "missing session", want: 303},
		{name: "invalid session", auth: fakeAuth{err: pgx.ErrNoRows}, cookie: true, want: 303},
		{name: "wrong organization", auth: fakeAuth{session: identity.Session{OrganizationID: "other", Role: "OWNER"}}, cookie: true, want: 403},
		{name: "no owner membership", auth: fakeAuth{session: identity.Session{OrganizationID: DemoOrganization, Role: "VIEWER"}}, cookie: true, want: 403},
		{name: "owner", auth: fakeAuth{session: identity.Session{UserID: "user-id", OrganizationID: DemoOrganization, Role: "OWNER"}}, cookie: true, want: 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := Server{Auth: tc.auth}
			handler := server.requireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if actor(r) != "user-id" {
					t.Fatal("actor missing from request context")
				}
				w.WriteHeader(204)
			}))
			request := httptest.NewRequest("GET", "http://localhost/app/dashboard", nil)
			if tc.cookie {
				request.AddCookie(&http.Cookie{Name: sessionCookie, Value: "session"})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.want {
				t.Fatalf("got %d want %d", response.Code, tc.want)
			}
		})
	}
}
