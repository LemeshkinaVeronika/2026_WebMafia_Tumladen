package postgres

import (
	"context"
	"database/sql"
	"github.com/webmafia/tumladan/internal/model"
	"time"
)

func (r *Repository) getRoomByIDForUpdate(ctx context.Context, tx *sql.Tx, roomID string) (*model.Room, error) {
	query := `
		SELECT id, name, is_private, invite_code, owner_actor_id, owner_actor_type, status, game_type, max_players, settings, last_empty_at, created_at, updated_at
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
		&room.OwnerActorType,
		&room.Status,
		&room.GameType,
		&room.MaxPlayers,
		&room.Settings,
		&room.LastEmptyAt,
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
		SELECT room_id, actor_id, actor_type, display_name, joined_at
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
			&p.ActorType,
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

func (r *Repository) addParticipantTx(ctx context.Context, tx *sql.Tx, roomID, actorID string, actorType model.ActorType, displayName string) error {
	query := `
		INSERT INTO room_participants (room_id, actor_id, actor_type, display_name, joined_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (room_id, actor_id) DO UPDATE
		SET actor_type = EXCLUDED.actor_type,
		    display_name = EXCLUDED.display_name
	`

	_, err := tx.ExecContext(ctx, query, roomID, actorID, actorType, displayName)
	return err
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
		INSERT INTO match_players (match_id, actor_id, actor_type, display_name, bot_difficulty, seat, disconnected_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	for _, player := range players {
		if _, err := tx.ExecContext(
			ctx,
			query,
			player.MatchID,
			player.ActorID,
			player.ActorType,
			player.DisplayName,
			player.BotDifficulty,
			player.Seat,
			player.DisconnectedAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) getActiveMatchByRoomIDForUpdate(ctx context.Context, tx *sql.Tx, roomID string) (*model.Match, error) {
	query := `
		SELECT id, room_id, game_type, status, game_state, result, termination_reason, terminated_by_actor_id, terminated_at, created_at, updated_at
		FROM matches
		WHERE room_id = $1 AND status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`

	var match model.Match
	err := tx.QueryRowContext(ctx, query, roomID).Scan(
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
		err = mapErrors(err)
		if err == ErrNotFound {
			return nil, ErrActiveMatchNotFound
		}
		return nil, err
	}

	return &match, nil
}

func (r *Repository) listMatchPlayersTx(ctx context.Context, tx *sql.Tx, matchID string) ([]model.MatchPlayer, error) {
	query := `
		SELECT mp.match_id, mp.actor_id, mp.actor_type, mp.display_name, COALESCE(mp.bot_difficulty, ''), COALESCE(u.avatar_url, ''), mp.seat, mp.disconnected_at
		FROM match_players mp
		LEFT JOIN users u ON mp.actor_type = 'user' AND u.id::TEXT = mp.actor_id
		WHERE mp.match_id = $1
		ORDER BY mp.seat ASC
	`

	rows, err := tx.QueryContext(ctx, query, matchID)
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
			&player.BotDifficulty,
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

func (r *Repository) terminateMatchTx(
	ctx context.Context,
	tx *sql.Tx,
	matchID string,
	state model.JSONB,
	result *model.JSONB,
	reason model.MatchTerminationReason,
	terminatedByActorID *string,
	terminatedAt time.Time,
) error {
	query := `
		UPDATE matches
		SET game_state = $2,
		    status = $3,
		    result = $4,
		    termination_reason = $5,
		    terminated_by_actor_id = $6,
		    terminated_at = $7,
		    updated_at = NOW()
		WHERE id = $1
	`

	res, err := tx.ExecContext(ctx, query, matchID, state, model.MatchStatusFinished, result, reason, terminatedByActorID, terminatedAt)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *Repository) updateRoomStatusTx(ctx context.Context, tx *sql.Tx, roomID string, status model.RoomStatus) (time.Time, error) {
	query := `
		UPDATE rooms
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	var updatedAt time.Time
	if err := tx.QueryRowContext(ctx, query, roomID, status).Scan(&updatedAt); err != nil {
		return time.Time{}, err
	}

	return updatedAt, nil
}
