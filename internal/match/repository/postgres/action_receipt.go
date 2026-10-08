package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) GetActionReceipt(ctx context.Context, roomID, actorID, actionID string) (*model.MatchActionReceipt, error) {
	const op = "match.repository.postgres.GetActionReceipt"

	const query = `
		SELECT room_id, match_id, actor_id, action_id, request_hash, status,
		       state_version, COALESCE(error_code, ''), COALESCE(error_message, ''), created_at
		FROM match_action_receipts
		WHERE room_id = $1 AND actor_id = $2 AND action_id = $3
	`

	var receipt model.MatchActionReceipt
	if err := r.db.QueryRowContext(ctx, query, roomID, actorID, actionID).Scan(
		&receipt.RoomID,
		&receipt.MatchID,
		&receipt.ActorID,
		&receipt.ActionID,
		&receipt.RequestHash,
		&receipt.Status,
		&receipt.StateVersion,
		&receipt.ErrorCode,
		&receipt.ErrorMessage,
		&receipt.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("[%s]: query failed: %w", op, mapErrors(err))
	}

	return &receipt, nil
}

func (r *Repository) SaveActionReceipt(ctx context.Context, receipt *model.MatchActionReceipt) error {
	const op = "match.repository.postgres.SaveActionReceipt"

	if receipt == nil {
		return fmt.Errorf("[%s]: nil action receipt", op)
	}
	if err := insertActionReceipt(ctx, r.db, receipt); err != nil {
		return fmt.Errorf("[%s]: insert failed: %w", op, mapActionReceiptError(err))
	}
	return nil
}

func (r *Repository) UpdateStateWithActionReceipt(
	ctx context.Context,
	matchID string,
	expectedStateVersion int,
	state model.JSONB,
	status model.MatchStatus,
	result *model.JSONB,
	receipt *model.MatchActionReceipt,
) error {
	const op = "match.repository.postgres.UpdateStateWithActionReceipt"

	if receipt == nil {
		return fmt.Errorf("[%s]: nil action receipt", op)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertActionReceipt(ctx, tx, receipt); err != nil {
		return fmt.Errorf("[%s]: insert receipt failed: %w", op, mapActionReceiptError(err))
	}

	const updateQuery = `
		UPDATE matches
		SET game_state = $2,
		    status = $3,
		    result = $4,
		    updated_at = NOW()
		WHERE id = $1
		  AND status = 'active'
		  AND (game_state->>'version')::BIGINT = $5
	`
	res, err := tx.ExecContext(ctx, updateQuery, matchID, state, status, result, expectedStateVersion)
	if err != nil {
		return fmt.Errorf("[%s]: update match failed: %w", op, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("[%s]: rows affected failed: %w", op, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("[%s]: update match failed: %w", op, ErrStateConflict)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[%s]: commit failed: %w", op, err)
	}
	return nil
}

type receiptExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertActionReceipt(ctx context.Context, execer receiptExecer, receipt *model.MatchActionReceipt) error {
	const query = `
		INSERT INTO match_action_receipts (
			room_id, match_id, actor_id, action_id, request_hash, status,
			state_version, error_code, error_message
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''))
	`
	_, err := execer.ExecContext(
		ctx,
		query,
		receipt.RoomID,
		receipt.MatchID,
		receipt.ActorID,
		receipt.ActionID,
		receipt.RequestHash,
		receipt.Status,
		receipt.StateVersion,
		receipt.ErrorCode,
		receipt.ErrorMessage,
	)
	return err
}

func mapActionReceiptError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrActionAlreadySaved
	}
	return err
}
