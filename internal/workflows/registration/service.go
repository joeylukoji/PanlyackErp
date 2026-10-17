package registration

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	identity "panlyackerp/internal/modules/identity/ports"
	"panlyackerp/internal/platform/db"
)

type Service struct {
	UOW      db.Store
	Identity identity.Auth
	Cash     interface {
		SeedDemo(context.Context, string, string) error
	}
	Accounting interface {
		SeedDemo(context.Context, string, string, string) error
	}
	Reporting interface {
		SeedDemo(context.Context, string) error
	}
}

// Register provisions an isolated demo workspace and its first session atomically.
func (s Service) Register(ctx context.Context, name, email, password, standard string) (string, error) {
	var token string
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context) error {
		orgID := uuid.NewString()
		sourceID := uuid.NewString()
		session, err := s.Identity.Register(ctx, orgID, name, email, password, standard)
		if err != nil {
			return fmt.Errorf("create workspace owner: %w", err)
		}
		if err := s.Cash.SeedDemo(ctx, orgID, sourceID); err != nil {
			return fmt.Errorf("seed cash: %w", err)
		}
		if err := s.Accounting.SeedDemo(ctx, orgID, sourceID, standard); err != nil {
			return fmt.Errorf("seed accounting: %w", err)
		}
		if err := s.Reporting.SeedDemo(ctx, orgID); err != nil {
			return fmt.Errorf("seed reporting: %w", err)
		}
		token, err = s.Identity.CreateSession(ctx, session)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		return nil
	})
	return token, err
}
