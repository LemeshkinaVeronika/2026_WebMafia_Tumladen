package postgres

import (
	"context"
	"fmt"

	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) AddParticipant(ctx context.Context, roomID, actorID, displayName string) error {
	const op = "room.repository.postgres.AddParticipant"

	query := `
		INSERT INTO room_participants (room_id, actor_id, display_name, joined_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (room_id, actor_id) DO UPDATE
		SET display_name = EXCLUDED.display_name
	`

	_, err := r.db.ExecContext(ctx, query, roomID, actorID, displayName)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	return nil
}

func (r *Repository) RemoveParticipant(ctx context.Context, roomID, actorID string) error {
	const op = "room.repository.postgres.RemoveParticipant"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	_, err = tx.ExecContext(ctx, `
		DELETE FROM room_participants
		WHERE room_id = $1 AND actor_id = $2
	`, roomID, actorID)
	if err != nil {
		return fmt.Errorf("[%s]: delete participant failed: %w", op, err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE rooms
		SET updated_at = NOW(),
			last_empty_at = CASE
				WHEN EXISTS (
					SELECT 1
					FROM room_participants
					WHERE room_id = $1
				) THEN NULL
				ELSE NOW()
			END
		WHERE id = $1`, roomID)
	if err != nil {
		return fmt.Errorf("[%s]: update updated_at failed: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return nil
}

func (r *Repository) ListParticipants(ctx context.Context, roomID string) ([]model.RoomParticipant, error) {
	const op = "room.repository.postgres.ListParticipants"

	query := `
		SELECT room_id, actor_id, display_name, joined_at
		FROM room_participants
		WHERE room_id = $1
		ORDER BY joined_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("[%s]: query failed: %w", op, err)
	}
	defer rows.Close()

	participants := make([]model.RoomParticipant, 0)
	for rows.Next() {
		var p model.RoomParticipant
		if err := rows.Scan(
			&p.RoomID,
			&p.ActorID,
			&p.DisplayName,
			&p.JoinedAt,
		); err != nil {
			return nil, fmt.Errorf("[%s]: scan failed: %w", op, err)
		}
		participants = append(participants, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[%s]: rows failed: %w", op, err)
	}

	return participants, nil
}

func (r *Repository) KickParticipant(ctx context.Context, roomID, targetActorID string) error {
	const op = "room.repository.postgres.KickParticipant"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	deleteQuery := `
		DELETE FROM room_participants
		WHERE room_id = $1 AND actor_id = $2
	`

	if _, err := tx.ExecContext(ctx, deleteQuery, roomID, targetActorID); err != nil {
		return fmt.Errorf("[%s]: delete participant failed: %w", op, err)
	}

	updateQuery := `
		UPDATE rooms
		SET updated_at = NOW(),
			last_empty_at = CASE
				WHEN EXISTS (
					SELECT 1
					FROM room_participants
					WHERE room_id = $1
				) THEN NULL
				ELSE NOW()
			END
		WHERE id = $1
	`

	if _, err := tx.ExecContext(ctx, updateQuery, roomID); err != nil {
		return fmt.Errorf("[%s]: update room timestamp failed: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return nil
}
