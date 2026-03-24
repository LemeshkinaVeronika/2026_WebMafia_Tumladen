package postgres

import (
	"context"
	"fmt"

	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) Create(ctx context.Context, room *model.Room) error {
	const op = "room.repository.postgres.Create"

	query := `
		INSERT INTO rooms (
			id, name, is_private, invite_code, owner_actor_id, status, game_type, max_players, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		room.ID,
		room.Name,
		room.IsPrivate,
		room.InviteCode,
		room.OwnerActorID,
		room.Status,
		room.GameType,
		room.MaxPlayers,
		room.CreatedAt,
		room.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	return nil
}

func (r *Repository) ListPublic(ctx context.Context) ([]model.Room, error) {
	const op = "room.repository.postgres.ListPublic"

	query := `
		SELECT id, name, is_private, invite_code, owner_actor_id, status, game_type, max_players, created_at, updated_at
		FROM rooms
		WHERE is_private = FALSE
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("[%s]: query failed: %w", op, err)
	}
	defer rows.Close()

	rooms := make([]model.Room, 0)
	for rows.Next() {
		var room model.Room
		if err := rows.Scan(
			&room.ID,
			&room.Name,
			&room.IsPrivate,
			&room.InviteCode,
			&room.OwnerActorID,
			&room.Status,
			&room.GameType,
			&room.MaxPlayers,
			&room.CreatedAt,
			&room.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("[%s]: scan failed: %w", op, err)
		}

		rooms = append(rooms, room)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[%s]: rows failed: %w", op, err)
	}

	return rooms, nil
}

func (r *Repository) GetByInviteCode(ctx context.Context, inviteCode string) (*model.Room, error) {
	const op = "room.repository.postgres.GetByInviteCode"

	query := `
		SELECT id, name, is_private, invite_code, owner_actor_id, status, game_type, max_players, created_at, updated_at
		FROM rooms
		WHERE invite_code = $1
		LIMIT 1
	`

	var room model.Room
	err := r.db.QueryRowContext(ctx, query, inviteCode).Scan(
		&room.ID,
		&room.Name,
		&room.IsPrivate,
		&room.InviteCode,
		&room.OwnerActorID,
		&room.Status,
		&room.GameType,
		&room.MaxPlayers,
		&room.CreatedAt,
		&room.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("[%s]: query failed: %w", op, mapErrors(err))
	}

	return &room, nil
}

func (r *Repository) GetByID(ctx context.Context, roomID string) (*model.Room, error) {
	const op = "room.repository.postgres.GetByID"

	query := `
		SELECT id, name, is_private, invite_code, owner_actor_id, status, game_type, max_players, created_at, updated_at
		FROM rooms
		WHERE id = $1
		LIMIT 1
	`

	var room model.Room
	err := r.db.QueryRowContext(ctx, query, roomID).Scan(
		&room.ID,
		&room.Name,
		&room.IsPrivate,
		&room.InviteCode,
		&room.OwnerActorID,
		&room.Status,
		&room.GameType,
		&room.MaxPlayers,
		&room.CreatedAt,
		&room.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("[%s]: query failed: %w", op, mapErrors(err))
	}

	return &room, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, roomID, gameType string, maxPlayers int) error {
	const op = "room.repository.postgres.UpdateSettings"

	query := `
		UPDATE rooms
		SET game_type = $2,
		    max_players = $3,
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, roomID, gameType, maxPlayers)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("[%s]: rows affected failed: %w", op, err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, roomID string, status model.RoomStatus) error {
	const op = "room.repository.postgres.UpdateStatus"

	query := `
		UPDATE rooms
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, roomID, status)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("[%s]: rows affected failed: %w", op, err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}
