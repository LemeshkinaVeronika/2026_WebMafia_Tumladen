package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) Create(ctx context.Context, session *model.GuestSession) error {
	const op = "guest.repository.postgres.Create"

	query := `
		INSERT INTO guest_sessions (session_id, actor_id, display_name, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.ExecContext(ctx, query, session.SessionID, session.ActorID, session.DisplayName, session.CreatedAt, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, mapErrors(err))
	}

	return nil
}

func (r *Repository) DeleteByActorID(ctx context.Context, actorID string, now time.Time) ([]string, error) {
	const op = "guest.repository.postgres.DeleteByActorID"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	roomIDs, err := deleteGuestParticipantsInWaitingRooms(ctx, tx, `
		rp.actor_id = $1
	`, actorID)
	if err != nil {
		return nil, fmt.Errorf("[%s]: delete participants failed: %w", op, err)
	}

	if err := updateWaitingRoomsEmptyState(ctx, tx, roomIDs, now); err != nil {
		return nil, fmt.Errorf("[%s]: update rooms failed: %w", op, err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM guest_sessions
		WHERE actor_id = $1
	`, actorID); err != nil {
		return nil, fmt.Errorf("[%s]: delete session failed: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return roomIDs, nil
}

func (r *Repository) CleanupExpired(ctx context.Context, now time.Time) ([]string, error) {
	const op = "guest.repository.postgres.CleanupExpired"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	roomIDs, err := deleteGuestParticipantsInWaitingRooms(ctx, tx, `
		NOT EXISTS (
			SELECT 1
			FROM guest_sessions gs
			WHERE gs.actor_id::TEXT = rp.actor_id
			  AND (gs.expires_at IS NULL OR gs.expires_at > $1)
		)
	`, now)
	if err != nil {
		return nil, fmt.Errorf("[%s]: delete participants failed: %w", op, err)
	}

	if err := updateWaitingRoomsEmptyState(ctx, tx, roomIDs, now); err != nil {
		return nil, fmt.Errorf("[%s]: update rooms failed: %w", op, err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM guest_sessions
		WHERE expires_at IS NOT NULL
		  AND expires_at <= $1
	`, now); err != nil {
		return nil, fmt.Errorf("[%s]: delete sessions failed: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return roomIDs, nil
}

func deleteGuestParticipantsInWaitingRooms(ctx context.Context, tx *sql.Tx, condition string, args ...any) ([]string, error) {
	query := fmt.Sprintf(`
		DELETE FROM room_participants rp
		USING rooms r
		WHERE rp.room_id = r.id
		  AND r.status = 'waiting'
		  AND rp.actor_type = 'guest'
		  AND %s
		RETURNING rp.room_id
	`, condition)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]struct{})
	roomIDs := make([]string, 0)
	for rows.Next() {
		var roomID string
		if err := rows.Scan(&roomID); err != nil {
			return nil, err
		}
		if _, ok := seen[roomID]; ok {
			continue
		}
		seen[roomID] = struct{}{}
		roomIDs = append(roomIDs, roomID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roomIDs, nil
}

func updateWaitingRoomsEmptyState(ctx context.Context, tx *sql.Tx, roomIDs []string, now time.Time) error {
	for _, roomID := range roomIDs {
		if _, err := tx.ExecContext(ctx, `
			UPDATE rooms
			SET updated_at = $2,
				last_empty_at = CASE
					WHEN EXISTS (
						SELECT 1
						FROM room_participants
						WHERE room_id = $1
					) THEN NULL
					ELSE COALESCE(last_empty_at, $2)
				END
			WHERE id = $1
			  AND status = 'waiting'
		`, roomID, now); err != nil {
			return err
		}
	}

	return nil
}
