package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/webmafia/tumladan/internal/model"
)

func (r *Repository) Create(ctx context.Context, room *model.Room) error {
	const op = "room.repository.postgres.Create"

	query := `
		INSERT INTO rooms (
			id, name, is_private, invite_code, owner_actor_id, owner_actor_type, status, game_type, max_players, settings, last_empty_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		room.ID,
		room.Name,
		room.IsPrivate,
		room.InviteCode,
		room.OwnerActorID,
		room.OwnerActorType,
		room.Status,
		room.GameType,
		room.MaxPlayers,
		room.Settings,
		room.LastEmptyAt,
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
		SELECT
			r.id,
			r.name,
			r.is_private,
			r.invite_code,
			r.owner_actor_id,
			r.owner_actor_type,
			r.status,
			r.game_type,
			r.max_players,
			COUNT(rp.actor_id) AS players_count,
			r.settings,
			r.last_empty_at,
			r.created_at,
			r.updated_at
		FROM rooms r
		LEFT JOIN room_participants rp ON rp.room_id = r.id
		WHERE r.is_private = FALSE
		GROUP BY
			r.id, r.name, r.is_private, r.invite_code, r.owner_actor_id, r.owner_actor_type,
			r.status, r.game_type, r.max_players, r.settings, r.last_empty_at, r.created_at, r.updated_at
		ORDER BY r.created_at DESC
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
			&room.OwnerActorType,
			&room.Status,
			&room.GameType,
			&room.MaxPlayers,
			&room.PlayersCount,
			&room.Settings,
			&room.LastEmptyAt,
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
		SELECT id, name, is_private, invite_code, owner_actor_id, owner_actor_type, status, game_type, max_players, settings, last_empty_at, created_at, updated_at
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
		return nil, fmt.Errorf("[%s]: query failed: %w", op, mapErrors(err))
	}

	return &room, nil
}

func (r *Repository) GetByID(ctx context.Context, roomID string) (*model.Room, error) {
	const op = "room.repository.postgres.GetByID"

	query := `
		SELECT id, name, is_private, invite_code, owner_actor_id, owner_actor_type, status, game_type, max_players, settings, last_empty_at, created_at, updated_at
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
		return nil, fmt.Errorf("[%s]: query failed: %w", op, mapErrors(err))
	}

	return &room, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, roomID, name string, isPrivate bool, gameType string, maxPlayers int, settings model.JSONB, reservedSlots int) (*model.Room, []model.RoomParticipant, error) {
	const op = "room.repository.postgres.UpdateRoomSettings"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	room, err := r.getRoomByIDForUpdate(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: get room for update failed: %w", op, err)
	}

	participants, err := r.listParticipantsTx(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list participants failed: %w", op, err)
	}

	if len(participants)+reservedSlots > maxPlayers {
		return nil, nil, ErrMaxPlayersLessThanParticipants
	}

	if room.Status != model.RoomStatusWaiting {
		return nil, nil, ErrRoomSettingsLocked
	}

	query := `
	UPDATE rooms
	SET name = $2,
	    is_private = $3,
	    game_type = $4,
	    max_players = $5,
	    settings = $6,
	    updated_at = NOW()
	WHERE id = $1
	RETURNING updated_at
`

	var updatedAt time.Time
	if err := tx.QueryRowContext(ctx, query, roomID, name, isPrivate, gameType, maxPlayers, settings).Scan(&updatedAt); err != nil {
		return nil, nil, fmt.Errorf("[%s]: update failed: %w", op, err)
	}

	room.Name = name
	room.IsPrivate = isPrivate
	room.GameType = gameType
	room.MaxPlayers = maxPlayers
	room.Settings = settings
	room.UpdatedAt = updatedAt
	room.PlayersCount = len(participants)

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return room, participants, nil
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

func (r *Repository) JoinRoom(ctx context.Context, roomID, actorID string, actorType model.ActorType, displayName string, reservedSlots int) (*model.Room, []model.RoomParticipant, error) {
	const op = "room.repository.postgres.JoinRoom"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	room, err := r.getRoomByIDForUpdate(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: get room for update failed: %w", op, err)
	}

	participants, err := r.listParticipantsTx(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list participants failed: %w", op, err)
	}

	alreadyInRoom := false
	for _, participant := range participants {
		if participant.ActorID == actorID {
			alreadyInRoom = true
			break
		}
	}

	if room.Status != model.RoomStatusWaiting && !(room.Status == model.RoomStatusPlaying && alreadyInRoom) {
		return nil, nil, ErrRoomNotJoinable
	}

	if !alreadyInRoom && len(participants)+reservedSlots >= room.MaxPlayers {
		return nil, nil, ErrRoomFull
	}

	if err := r.addParticipantTx(ctx, tx, roomID, actorID, actorType, displayName); err != nil {
		return nil, nil, fmt.Errorf("[%s]: add participant failed: %w", op, err)
	}

	_, err = tx.ExecContext(ctx, `
    UPDATE rooms
    SET updated_at = NOW(),
     last_empty_at = NULL
    WHERE id = $1`, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: update updated_at failed: %w", op, err)
	}

	participants, err = r.listParticipantsTx(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list participants after insert failed: %w", op, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	room.PlayersCount = len(participants)

	return room, participants, nil
}

func (r *Repository) StartRoomWithMatch(ctx context.Context, roomID string, match *model.Match, players []model.MatchPlayer) (*model.Room, []model.RoomParticipant, error) {
	const op = "room.repository.postgres.StartRoomWithMatch"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	room, err := r.getRoomByIDForUpdate(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: get room for update failed: %w", op, err)
	}

	if room.Status != model.RoomStatusWaiting {
		return nil, nil, ErrRoomNotReady
	}

	participants, err := r.listParticipantsTx(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list participants failed: %w", op, err)
	}

	if len(participants) == 0 || len(players) < 2 {
		return nil, nil, ErrNotEnoughPlayers
	}

	if len(players) > room.MaxPlayers {
		return nil, nil, ErrRoomFull
	}

	if !matchPlayersMatchParticipants(players, participants) {
		return nil, nil, ErrRoomNotReady
	}

	if err := r.createMatchTx(ctx, tx, match); err != nil {
		return nil, nil, fmt.Errorf("[%s]: create match failed: %w", op, err)
	}

	if err := r.createMatchPlayersTx(ctx, tx, players); err != nil {
		return nil, nil, fmt.Errorf("[%s]: create match players failed: %w", op, err)
	}

	query := `
		UPDATE rooms
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	var updatedAt time.Time
	if err := tx.QueryRowContext(ctx, query, roomID, model.RoomStatusPlaying).Scan(&updatedAt); err != nil {
		return nil, nil, fmt.Errorf("[%s]: update room status failed: %w", op, err)
	}

	room.Status = model.RoomStatusPlaying
	room.UpdatedAt = updatedAt
	room.PlayersCount = len(participants)

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return room, participants, nil
}

func matchPlayersMatchParticipants(players []model.MatchPlayer, participants []model.RoomParticipant) bool {
	participantsByActorID := make(map[string]struct{}, len(participants))
	for _, participant := range participants {
		participantsByActorID[participant.ActorID] = struct{}{}
	}

	realPlayerCount := 0
	for _, player := range players {
		if player.ActorType == model.ActorTypeBot {
			continue
		}
		if _, ok := participantsByActorID[player.ActorID]; !ok {
			return false
		}
		realPlayerCount++
	}
	return realPlayerCount == len(participants)
}

func (r *Repository) TerminateActiveMatch(
	ctx context.Context,
	roomID string,
	state *model.JSONB,
	reason model.MatchTerminationReason,
	result *model.JSONB,
	terminatedByActorID *string,
	terminatedAt time.Time,
) (*model.Match, []model.MatchPlayer, error) {
	const op = "room.repository.postgres.TerminateActiveMatch"

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: begin tx failed: %w", op, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	room, err := r.getRoomByIDForUpdate(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: get room for update failed: %w", op, err)
	}

	match, err := r.getActiveMatchByRoomIDForUpdate(ctx, tx, roomID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: get active match failed: %w", op, err)
	}

	players, err := r.listMatchPlayersTx(ctx, tx, match.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: list match players failed: %w", op, err)
	}

	nextState := match.GameState
	if state != nil {
		nextState = *state
	}

	if err := r.terminateMatchTx(ctx, tx, match.ID, nextState, result, reason, terminatedByActorID, terminatedAt); err != nil {
		return nil, nil, fmt.Errorf("[%s]: update match failed: %w", op, err)
	}

	updatedAt, err := r.updateRoomStatusTx(ctx, tx, roomID, model.RoomStatusWaiting)
	if err != nil {
		return nil, nil, fmt.Errorf("[%s]: update room status failed: %w", op, err)
	}

	match.Status = model.MatchStatusFinished
	match.GameState = nextState
	match.Result = result
	match.TerminationReason = &reason
	match.TerminatedByActorID = terminatedByActorID
	match.TerminatedAt = &terminatedAt
	match.UpdatedAt = terminatedAt

	room.Status = model.RoomStatusWaiting
	room.UpdatedAt = updatedAt
	_ = room

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("[%s]: commit failed: %w", op, err)
	}

	return match, players, nil
}

func (r *Repository) MarkActiveMatchPlayerDisconnected(ctx context.Context, roomID, actorID string, disconnectedAt time.Time) error {
	const op = "room.repository.postgres.MarkActiveMatchPlayerDisconnected"

	query := `
		UPDATE match_players mp
		SET disconnected_at = COALESCE(mp.disconnected_at, $3)
		FROM matches m
		WHERE mp.match_id = m.id
		  AND m.room_id = $1
		  AND m.status = 'active'
		  AND mp.actor_id = $2
	`

	if _, err := r.db.ExecContext(ctx, query, roomID, actorID, disconnectedAt); err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	return nil
}

func (r *Repository) MarkActiveMatchPlayerConnected(ctx context.Context, roomID, actorID string) error {
	const op = "room.repository.postgres.MarkActiveMatchPlayerConnected"

	query := `
		UPDATE match_players mp
		SET disconnected_at = NULL
		FROM matches m
		WHERE mp.match_id = m.id
		  AND m.room_id = $1
		  AND m.status = 'active'
		  AND mp.actor_id = $2
	`

	if _, err := r.db.ExecContext(ctx, query, roomID, actorID); err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	return nil
}

func (r *Repository) DeleteRoom(ctx context.Context, roomID string) error {
	const op = "room.repository.postgres.DeleteRoom"

	query := `
		DELETE FROM rooms
		WHERE id = $1
		  AND status != 'playing'
	`

	res, err := r.db.ExecContext(ctx, query, roomID)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("[%s]: rows affected failed: %w", op, err)
	}
	if rowsAffected == 0 {
		room, getErr := r.GetByID(ctx, roomID)
		switch {
		case getErr == nil && room.Status == model.RoomStatusPlaying:
			return ErrForbiddenDeleteActiveRoom
		case getErr == nil:
			return ErrNotFound
		case errors.Is(getErr, ErrNotFound):
			return ErrNotFound
		default:
			return fmt.Errorf("[%s]: check room state failed: %w", op, getErr)
		}
	}

	return nil
}

func (r *Repository) FindStaleEmptyWaitingRooms(ctx context.Context, olderThan time.Time) ([]model.Room, error) {
	const op = "room.repository.postgres.FindStaleEmptyWaitingRooms"

	query := `
		SELECT
			r.id,
			r.name,
			r.is_private,
			r.invite_code,
			r.owner_actor_id,
			r.owner_actor_type,
			r.status,
			r.game_type,
			r.max_players,
			r.settings,
			r.last_empty_at,
			r.created_at,
			r.updated_at
		FROM rooms r
		LEFT JOIN room_participants rp ON rp.room_id = r.id
		WHERE r.status = 'waiting'
		  AND rp.room_id IS NULL
		  AND r.last_empty_at IS NOT NULL
		  AND r.last_empty_at < $1
		ORDER BY r.last_empty_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, olderThan)
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
			&room.OwnerActorType,
			&room.Status,
			&room.GameType,
			&room.MaxPlayers,
			&room.Settings,
			&room.LastEmptyAt,
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

func (r *Repository) FindRoomsWithReconnectTimeout(ctx context.Context, olderThan time.Time) ([]model.Room, error) {
	const op = "room.repository.postgres.FindRoomsWithReconnectTimeout"

	query := `
		SELECT
			r.id,
			r.name,
			r.is_private,
			r.invite_code,
			r.owner_actor_id,
			r.owner_actor_type,
			r.status,
			r.game_type,
			r.max_players,
			r.settings,
			r.last_empty_at,
			r.created_at,
			r.updated_at
		FROM rooms r
		WHERE r.status = 'playing'
		  AND EXISTS (
			SELECT 1
			FROM matches m
			JOIN match_players mp ON mp.match_id = m.id
			WHERE m.room_id = r.id
			  AND m.status = 'active'
			  AND mp.disconnected_at IS NOT NULL
			  AND mp.disconnected_at < $1
		  )
		ORDER BY r.updated_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, olderThan)
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
			&room.OwnerActorType,
			&room.Status,
			&room.GameType,
			&room.MaxPlayers,
			&room.Settings,
			&room.LastEmptyAt,
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

func (r *Repository) MarkRoomEmpty(ctx context.Context, roomID string) error {
	const op = "room.repository.postgres.MarkRoomEmpty"

	query := `
		UPDATE rooms
		SET last_empty_at = COALESCE(last_empty_at, NOW()),
    	updated_at = NOW()
		WHERE id = $1
		  AND status = 'playing'
	`

	_, err := r.db.ExecContext(ctx, query, roomID)
	if err != nil {
		return fmt.Errorf("[%s]: exec failed: %w", op, err)
	}

	return nil
}
