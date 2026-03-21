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

	query := `
		DELETE FROM room_participants
		WHERE room_id = $1 AND actor_id = $2
	`

	_, err := r.db.ExecContext(ctx, query, roomID, actorID)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	return nil
}

func (r *Repository) ListParticipants(ctx context.Context, roomID string) ([]model.RoomParticipantView, error) {
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

	participants := make([]model.RoomParticipantView, 0)
	for rows.Next() {
		var p model.RoomParticipantView
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
