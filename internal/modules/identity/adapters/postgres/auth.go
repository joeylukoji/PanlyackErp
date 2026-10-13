package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"panlyackerp/internal/modules/identity/domain"
	"panlyackerp/internal/modules/identity/ports"
	"panlyackerp/internal/platform/db"
)

type AuthRepository struct{ DB db.Store }

// SetOwner is an explicit local bootstrap operation. It also revokes old sessions on password rotation.
func (r AuthRepository) SetOwner(ctx context.Context, organizationID, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 || !strings.Contains(email, "@") {
		return errors.New("valid owner email required")
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		return err
	}
	return r.DB.WithinTransaction(ctx, func(ctx context.Context) error {
		var id string
		err := r.DB.Conn(ctx).QueryRow(ctx, `INSERT INTO identity.users(id,email,password_hash) VALUES($1,$2,$3)
            ON CONFLICT(email) DO UPDATE SET password_hash=EXCLUDED.password_hash RETURNING id::text`, uuid.NewString(), email, hash).Scan(&id)
		if err != nil {
			return err
		}
		_, err = r.DB.Conn(ctx).Exec(ctx, `INSERT INTO identity.memberships(organization_id,user_id,role) VALUES($1,$2,'OWNER')
            ON CONFLICT(organization_id,user_id) DO UPDATE SET role='OWNER'`, organizationID, id)
		if err != nil {
			return err
		}
		_, err = r.DB.Conn(ctx).Exec(ctx, `DELETE FROM identity.sessions WHERE user_id=$1`, id)
		return err
	})
}

func (r AuthRepository) Authenticate(ctx context.Context, organizationID, email, password string) (ports.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 || len(password) > 128 {
		return ports.Session{}, ports.ErrInvalidCredentials
	}
	var session ports.Session
	var hash string
	err := r.DB.Conn(ctx).QueryRow(ctx, `SELECT u.id::text,m.organization_id::text,u.email,m.role,u.password_hash
        FROM identity.users u JOIN identity.memberships m ON m.user_id=u.id
        WHERE m.organization_id=$1 AND u.email=$2`, organizationID, email).Scan(&session.UserID, &session.OrganizationID, &session.Email, &session.Role, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		// Do equivalent key derivation for an unknown user to reduce account enumeration.
		domain.VerifyPassword("argon2id-v1$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", password)
		return ports.Session{}, ports.ErrInvalidCredentials
	}
	if err != nil {
		return ports.Session{}, err
	}
	if !domain.VerifyPassword(hash, password) {
		return ports.Session{}, ports.ErrInvalidCredentials
	}
	return session, nil
}

func (r AuthRepository) CreateSession(ctx context.Context, session ports.Session) (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	token := hex.EncodeToString(data)
	digest := sha256.Sum256([]byte(token))
	_, err := r.DB.Conn(ctx).Exec(ctx, `INSERT INTO identity.sessions(token_hash,user_id,organization_id,expires_at)
        VALUES($1,$2,$3,$4)`, digest[:], session.UserID, session.OrganizationID, time.Now().Add(8*time.Hour))
	return token, err
}

func (r AuthRepository) Session(ctx context.Context, token string) (ports.Session, error) {
	if len(token) != 64 {
		return ports.Session{}, pgx.ErrNoRows
	}
	if _, err := hex.DecodeString(token); err != nil {
		return ports.Session{}, pgx.ErrNoRows
	}
	digest := sha256.Sum256([]byte(token))
	var session ports.Session
	err := r.DB.Conn(ctx).QueryRow(ctx, `SELECT s.user_id::text,s.organization_id::text,u.email,m.role
        FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id
        JOIN identity.memberships m ON m.organization_id=s.organization_id AND m.user_id=s.user_id
        WHERE s.token_hash=$1 AND s.expires_at>now()`, digest[:]).Scan(&session.UserID, &session.OrganizationID, &session.Email, &session.Role)
	return session, err
}

func (r AuthRepository) RevokeSession(ctx context.Context, token string) error {
	if len(token) != 64 {
		return nil
	}
	digest := sha256.Sum256([]byte(token))
	_, err := r.DB.Conn(ctx).Exec(ctx, `DELETE FROM identity.sessions WHERE token_hash=$1`, digest[:])
	return err
}
