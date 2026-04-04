package postgres

import (
	"context"
	"database/sql"
	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) getRoomByIDForUpdate(ctx context.Context, tx *sql.Tx, roomID string) (*model.Room, error) {
	query := `
		SELECT id, name, is_private, invite_code, owner_actor_id, status, game_type, max_players, settings, created_at, updated_at
		FROM rooms
		WHERE id = $1
		FOR UPDATE
	`

	var room model.Room
	err := tx.QueryRowContext(ctx, query, roomID).Scan(
		&room.ID,
		&room.Name,
		&room.IsPrivate,
		&room.InviteCode,
		&room.OwnerActorID,
		&room.Status,
		&room.GameType,
		&room.MaxPlayers,
		&room.Settings,
		&room.CreatedAt,
		&room.UpdatedAt,
	)
	if err != nil {
		return nil, mapErrors(err)
	}

	return &room, nil
}

func (r *Repository) listParticipantsTx(ctx context.Context, tx *sql.Tx, roomID string) ([]model.RoomParticipant, error) {
	query := `
		SELECT room_id, actor_id, display_name, joined_at
		FROM room_participants
		WHERE room_id = $1
		ORDER BY joined_at ASC
	`

	rows, err := tx.QueryContext(ctx, query, roomID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		participants = append(participants, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return participants, nil
}

func (r *Repository) addParticipantTx(ctx context.Context, tx *sql.Tx, roomID, actorID, displayName string) error {
	query := `
		INSERT INTO room_participants (room_id, actor_id, display_name, joined_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (room_id, actor_id) DO UPDATE
		SET display_name = EXCLUDED.display_name
	`

	_, err := tx.ExecContext(ctx, query, roomID, actorID, displayName)
	return err
}
