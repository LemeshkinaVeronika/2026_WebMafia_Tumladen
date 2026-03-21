package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/webmafia/tumladan/internal/model"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, room *model.Room) error {
	const op = "room.repository.postgres.Create"

	query := `
		INSERT INTO rooms (id, name, is_private, invite_code, owner_actor_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
		SELECT id, name, is_private, invite_code, owner_actor_id, status, created_at, updated_at
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
