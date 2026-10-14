package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
			ON CONFLICT(email) DO UPDATE SET password_hash=EXCLUDED.password_hash
			WHERE EXISTS (SELECT 1 FROM identity.memberships WHERE user_id=identity.users.id AND organization_id=$4)
			RETURNING id::text`, uuid.NewString(), email, hash, organizationID).Scan(&id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("owner email belongs to another organization")
			}
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

func (r AuthRepository) Register(ctx context.Context, organizationID, name, email, password string) (ports.Session, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	address, err := mail.ParseAddress(email)
	if len(name) < 2 || len(name) > 80 || strings.IndexFunc(name, unicode.IsControl) >= 0 ||
		err != nil || address.Address != email || len(email) > 254 {
		return ports.Session{}, errors.New("invalid registration details")
	}
	hash, err := domain.HashPassword(password)
	if err != nil {
		return ports.Session{}, err
	}
	userID := uuid.NewString()
	conn := r.DB.Conn(ctx)
	if _, err = conn.Exec(ctx, `INSERT INTO identity.organizations(id,name,currency,accounting_standard,inventory_method)
		VALUES($1,$2,'USD','FR_PCG','FIFO')`, organizationID, name); err != nil {
		return ports.Session{}, err
	}
	if _, err = conn.Exec(ctx, `INSERT INTO identity.users(id,email,password_hash) VALUES($1,$2,$3)`, userID, email, hash); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ports.Session{}, ports.ErrEmailExists
		}
		return ports.Session{}, err
	}
	if _, err = conn.Exec(ctx, `INSERT INTO identity.memberships(organization_id,user_id,role)
		VALUES($1,$2,'OWNER')`, organizationID, userID); err != nil {
		return ports.Session{}, err
	}
	return ports.Session{
		UserID: userID, OrganizationID: organizationID,
		OrganizationName: name, Email: email, Role: "OWNER",
	}, nil
}

func (r AuthRepository) Authenticate(ctx context.Context, email, password string) (ports.Session, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) > 254 || len(password) > 128 {
		return ports.Session{}, ports.ErrInvalidCredentials
	}
	var session ports.Session
	var hash string
	err := r.DB.Conn(ctx).QueryRow(ctx, `SELECT u.id::text,m.organization_id::text,o.name,u.email,m.role,u.password_hash
		FROM identity.users u JOIN identity.memberships m ON m.user_id=u.id
		JOIN identity.organizations o ON o.id=m.organization_id
		WHERE u.email=$1`, email).Scan(&session.UserID, &session.OrganizationID, &session.OrganizationName, &session.Email, &session.Role, &hash)
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
	err := r.DB.Conn(ctx).QueryRow(ctx, `SELECT s.user_id::text,s.organization_id::text,o.name,u.email,m.role
        FROM identity.sessions s JOIN identity.users u ON u.id=s.user_id
        JOIN identity.memberships m ON m.organization_id=s.organization_id AND m.user_id=s.user_id
		JOIN identity.organizations o ON o.id=s.organization_id
		WHERE s.token_hash=$1 AND s.expires_at>now()`, digest[:]).Scan(&session.UserID, &session.OrganizationID, &session.OrganizationName, &session.Email, &session.Role)
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
