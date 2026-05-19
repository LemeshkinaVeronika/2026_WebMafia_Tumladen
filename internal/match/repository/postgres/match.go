package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) Create(ctx context.Context, match *model.Match, players []model.MatchPlayer) error {
	const op = "match.repository.postgres.Create"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := r.createMatchTx(ctx, tx, match); err != nil {
		return fmt.Errorf("[%s]: create match failed: %w", op, err)
	}

	if err := r.createMatchPlayersTx(ctx, tx, players); err != nil {
		return fmt.Errorf("[%s]: create match players failed: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return nil
}

func (r *Repository) createMatchTx(ctx context.Context, tx *sql.Tx, match *model.Match) error {
	query := `
		INSERT INTO matches (
			id, room_id, game_type, status, game_state, result, termination_reason, terminated_by_actor_id, terminated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := tx.ExecContext(
		ctx,
		query,
		match.ID,
		match.RoomID,
		match.GameType,
		match.Status,
		match.GameState,
		match.Result,
		match.TerminationReason,
		match.TerminatedByActorID,
		match.TerminatedAt,
		match.CreatedAt,
		match.UpdatedAt,
	)

	return err
}

func (r *Repository) createMatchPlayersTx(ctx context.Context, tx *sql.Tx, players []model.MatchPlayer) error {
	query := `
		INSERT INTO match_players (match_id, actor_id, actor_type, display_name, seat)
		VALUES ($1, $2, $3, $4, $5)
	`

	for _, player := range players {
		if _, err := tx.ExecContext(
			ctx,
			query,
			player.MatchID,
			player.ActorID,
			player.ActorType,
			player.DisplayName,
			player.Seat,
		); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) GetByID(ctx context.Context, matchID string) (*model.Match, []model.MatchPlayer, error) {
	const op = "match.repository.postgres.GetByID"

	match, err := r.getMatchByID(ctx, matchID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: get match failed: %w", op, err)
	}

	players, err := r.listMatchPlayers(ctx, matchID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list players failed: %w", op, err)
	}

	return match, players, nil
}

func (r *Repository) GetActiveByRoomID(ctx context.Context, roomID string) (*model.Match, []model.MatchPlayer, error) {
	const op = "match.repository.postgres.GetActiveByRoomID"

	query := `
		SELECT id, room_id, game_type, status, game_state, result, termination_reason, terminated_by_actor_id, terminated_at, created_at, updated_at
		FROM matches
		WHERE room_id = $1 AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`

	var match model.Match
	err := r.db.QueryRowContext(ctx, query, roomID).Scan(
		&match.ID,
		&match.RoomID,
		&match.GameType,
		&match.Status,
		&match.GameState,
		&match.Result,
		&match.TerminationReason,
		&match.TerminatedByActorID,
		&match.TerminatedAt,
		&match.CreatedAt,
		&match.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: query failed: %w", op, mapErrors(err))
	}

	players, err := r.listMatchPlayers(ctx, match.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list players failed: %w", op, err)
	}

	return &match, players, nil
}

func (r *Repository) getMatchByID(ctx context.Context, matchID string) (*model.Match, error) {
	query := `
		SELECT id, room_id, game_type, status, game_state, result, termination_reason, terminated_by_actor_id, terminated_at, created_at, updated_at
		FROM matches
		WHERE id = $1
		LIMIT 1
	`

	var match model.Match
	err := r.db.QueryRowContext(ctx, query, matchID).Scan(
		&match.ID,
		&match.RoomID,
		&match.GameType,
		&match.Status,
		&match.GameState,
		&match.Result,
		&match.TerminationReason,
		&match.TerminatedByActorID,
		&match.TerminatedAt,
		&match.CreatedAt,
		&match.UpdatedAt,
	)
	if err != nil {
		return nil, mapErrors(err)
	}

	return &match, nil
}

func (r *Repository) listMatchPlayers(ctx context.Context, matchID string) ([]model.MatchPlayer, error) {
	query := `
		SELECT mp.match_id, mp.actor_id, mp.actor_type, mp.display_name, COALESCE(u.avatar_url, ''), mp.seat, mp.disconnected_at
		FROM match_players mp
		LEFT JOIN users u ON mp.actor_type = 'user' AND u.id = mp.actor_id
		WHERE mp.match_id = $1
		ORDER BY mp.seat ASC
	`

	rows, err := r.db.QueryContext(ctx, query, matchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	players := make([]model.MatchPlayer, 0)
	for rows.Next() {
		var player model.MatchPlayer
		if err := rows.Scan(
			&player.MatchID,
			&player.ActorID,
			&player.ActorType,
			&player.DisplayName,
			&player.AvatarURL,
			&player.Seat,
			&player.DisconnectedAt,
		); err != nil {
			return nil, err
		}
		players = append(players, player)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return players, nil
}

func (r *Repository) UpdateState(ctx context.Context, matchID string, state model.JSONB, status model.MatchStatus, result *model.JSONB) error {
	const op = "match.repository.postgres.UpdateState"

	query := `
		UPDATE matches
		SET game_state = $2,
		    status = $3,
		    result = $4,
		    updated_at = NOW()
		WHERE id = $1 AND status = 'active'
	`

	res, err := r.db.ExecContext(ctx, query, matchID, state, status, result)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("[%s]: rows affected failed: %w", op, err)
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) GetLastByRoomID(ctx context.Context, roomID string) (*model.Match, []model.MatchPlayer, error) {
	const op = "match.repository.postgres.GetLastByRoomID"

	query := `
		SELECT id, room_id, game_type, status, game_state, result, termination_reason, terminated_by_actor_id, terminated_at, created_at, updated_at
		FROM matches
		WHERE room_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var match model.Match
	err := r.db.QueryRowContext(ctx, query, roomID).Scan(
		&match.ID,
		&match.RoomID,
		&match.GameType,
		&match.Status,
		&match.GameState,
		&match.Result,
		&match.TerminationReason,
		&match.TerminatedByActorID,
		&match.TerminatedAt,
		&match.CreatedAt,
		&match.UpdatedAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: query failed: %w", op, mapErrors(err))
	}

	players, err := r.listMatchPlayers(ctx, match.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list players failed: %w", op, err)
	}

	return &match, players, nil
}
