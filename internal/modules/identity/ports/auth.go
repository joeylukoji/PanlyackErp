package ports

import (
	"context"
	"errors"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Session struct {
	UserID         string
	OrganizationID string
	Email          string
	Role           string
}

type Auth interface {
	Authenticate(context.Context, string, string, string) (Session, error)
	CreateSession(context.Context, Session) (string, error)
	Session(context.Context, string) (Session, error)
	RevokeSession(context.Context, string) error
}
