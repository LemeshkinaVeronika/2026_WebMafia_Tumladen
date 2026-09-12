package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/webmafia/tumladan/internal/outbox"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func InsertTx(ctx context.Context, tx *sql.Tx, event outbox.Event) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (id, topic, event_key, event_type, payload, created_at, available_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
	`, event.ID, event.Topic, event.Key, event.Type, event.Payload, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (r *Repository) ClaimBatch(ctx context.Context, limit int, lease time.Duration) ([]outbox.Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND available_at <= NOW()
			  AND (locked_until IS NULL OR locked_until < NOW())
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE outbox_events e
		SET locked_until = NOW() + ($2 * INTERVAL '1 millisecond'),
		    attempts = attempts + 1
		FROM candidates c
		WHERE e.id = c.id
		RETURNING e.id, e.topic, e.event_key, e.event_type, e.payload, e.attempts, e.created_at
	`, limit, lease.Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("claim outbox batch: %w", err)
	}
	defer rows.Close()

	events := make([]outbox.Event, 0, limit)
	for rows.Next() {
		var event outbox.Event
		if err := rows.Scan(&event.ID, &event.Topic, &event.Key, &event.Type, &event.Payload, &event.Attempts, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

func (r *Repository) MarkPublished(ctx context.Context, eventID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET published_at = NOW(), locked_until = NULL, last_error = NULL
		WHERE id = $1 AND published_at IS NULL
	`, eventID)
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailed(ctx context.Context, eventID, message string, retryAfter time.Duration) error {
	if len(message) > 2000 {
		message = message[:2000]
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET locked_until = NULL,
		    available_at = NOW() + ($2 * INTERVAL '1 millisecond'),
		    last_error = $3
		WHERE id = $1 AND published_at IS NULL
	`, eventID, retryAfter.Milliseconds(), message)
	if err != nil {
		return fmt.Errorf("mark outbox event failed: %w", err)
	}
	return nil
}
