package postgres

import (
	"context"
	"fmt"

	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) Create(ctx context.Context, session *model.GuestSession) error {
	const op = "guest.repository.postgres.Create"

	query := `
		INSERT INTO guest_sessions (actor_id, display_name, created_at, expires_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		session.ActorID,
		session.DisplayName,
		session.CreatedAt,
		session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, mapErrors(err))
	}

	return nil
}
