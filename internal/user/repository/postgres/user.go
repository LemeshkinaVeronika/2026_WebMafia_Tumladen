package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/webmafia/tumladan/internal/model"
)

func (m *Repository) CreateUser(ctx context.Context, user model.User) error {
	const op = "user.repository.postgres.CreateUser"
	query := `INSERT INTO users (id, nickname, email, password_hash, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := m.Conn.ExecContext(ctx,
		query,
		user.ID, user.Nickname, user.Email, user.PasswordHash,
		user.AvatarURL, user.CreatedAt, user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("%s: %w", op, handlePostgresError(err))
	}
	return nil
}

func (m *Repository) GetUserByEmail(ctx context.Context, email string) (res *model.User, err error) {
	const op = "user.repository.postgres.GetUserByEmail"
	query := `SELECT id, nickname, email, password_hash, avatar_url, created_at, updated_at 
		FROM users WHERE email = $1`

	user, err := m.selectUser(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}

func (m *Repository) GetUserByLogin(ctx context.Context, login string) (res *model.User, err error) {
	const op = "user.repository.postgres.GetUserByLogin"

	query := `SELECT id, nickname, email, password_hash, avatar_url, created_at, updated_at 
		FROM users WHERE nickname = $1`

	user, err := m.selectUser(ctx, query, login)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}

func (m *Repository) UpdateUserAvatar(ctx context.Context, userID string, avatarPath string) error {
	const op = "user.repository.postgres.UpdateUserAvatar"

	query := `UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`
	res, err := m.Conn.ExecContext(ctx, query, avatarPath, userID)
	if err != nil {
		return fmt.Errorf("%s: %w", op, handlePostgresError(err))
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return fmt.Errorf("%s: user not found: %w", op, ErrNotFound)
	}
	return nil
}

func (m *Repository) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	const op = "user.repository.postgres.GetUserByID"

	query := `SELECT id, nickname, email, password_hash, avatar_url, created_at, updated_at 
			  FROM users WHERE id = $1`

	user, err := m.selectUser(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return user, nil
}

func (m *Repository) GetCurrentRoomByUserID(ctx context.Context, userID string) (*model.CurrentRoom, error) {
	const op = "user.repository.postgres.GetCurrentRoomByUserID"

	query := `
		SELECT
			r.id,
			r.name,
			r.invite_code,
			r.status,
			r.game_type,
			m.id
		FROM room_participants rp
		JOIN rooms r ON r.id = rp.room_id
		LEFT JOIN matches m ON m.room_id = r.id AND m.status = 'active'
		WHERE rp.actor_id = $1
		  AND rp.actor_type = 'user'
		  AND r.status IN ('waiting', 'playing')
		ORDER BY r.updated_at DESC
		LIMIT 1
	`

	var room model.CurrentRoom
	err := m.Conn.QueryRowContext(ctx, query, userID).Scan(
		&room.ID,
		&room.Name,
		&room.InviteCode,
		&room.Status,
		&room.GameType,
		&room.MatchID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: query current room: %w", op, err)
	}

	return &room, nil
}

func (m *Repository) ListFinishedMatchesByUserID(ctx context.Context, userID string) ([]model.MatchWithPlayers, error) {
	const op = "user.repository.postgres.ListFinishedMatchesByUserID"

	query := `
		SELECT
			m.id,
			m.room_id,
			m.game_type,
			m.status,
			m.game_state,
			m.result,
			m.termination_reason,
			m.terminated_by_actor_id,
			m.terminated_at,
			m.created_at,
			m.updated_at
		FROM matches m
		WHERE m.status = 'finished'
		  AND EXISTS (
			  SELECT 1
			  FROM match_players mp
			  WHERE mp.match_id = m.id
			    AND mp.actor_id = $1
			    AND mp.actor_type = 'user'
		  )
		ORDER BY COALESCE(m.terminated_at, m.updated_at, m.created_at) DESC
	`

	rows, err := m.Conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: query matches: %w", op, err)
	}
	defer rows.Close()

	matches := make([]model.MatchWithPlayers, 0)
	for rows.Next() {
		var match model.Match
		if err := rows.Scan(
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
		); err != nil {
			return nil, fmt.Errorf("%s: scan match: %w", op, err)
		}

		players, err := m.listMatchPlayers(ctx, match.ID)
		if err != nil {
			return nil, fmt.Errorf("%s: list match players: %w", op, err)
		}

		matches = append(matches, model.MatchWithPlayers{
			Match:   match,
			Players: players,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows: %w", op, err)
	}

	return matches, nil
}

func (m *Repository) listMatchPlayers(ctx context.Context, matchID string) ([]model.MatchPlayer, error) {
	query := `
		SELECT mp.match_id, mp.actor_id, mp.actor_type, mp.display_name, COALESCE(u.avatar_url, ''), mp.seat, mp.disconnected_at
		FROM match_players mp
		LEFT JOIN users u ON mp.actor_type = 'user' AND u.id = mp.actor_id
		WHERE mp.match_id = $1
		ORDER BY mp.seat ASC
	`

	rows, err := m.Conn.QueryContext(ctx, query, matchID)
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

func (m *Repository) UpdateUserProfile(ctx context.Context, user model.User) error {
	const op = "user.repository.postgres.UpdateUserProfile"

	query := `
		UPDATE users
		SET nickname = $1,
			email = $2,
			password_hash = COALESCE(NULLIF($3, ''), password_hash),
			updated_at = NOW()
		WHERE id = $4`

	res, err := m.Conn.ExecContext(ctx, query,
		user.Nickname,
		user.Email,
		user.PasswordHash,
		user.ID,
	)

	if err != nil {
		return fmt.Errorf("%s: %w", op, handlePostgresError(err))
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return fmt.Errorf("%s: user not found: %w", op, ErrNotFound)
	}
	return nil
}

func (m *Repository) selectUser(ctx context.Context, query string, args ...interface{}) (*model.User, error) {
	rows := m.Conn.QueryRowContext(ctx, query, args...)
	user := &model.User{}
	err := rows.Scan(
		&user.ID,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarURL,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("user not found: %w", ErrNotFound)
	}

	if err != nil {
		return nil, err
	}

	return user, nil
}

func handlePostgresError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("user already exist: %w", ErrConflict)
		default:
			return fmt.Errorf("postgres error: %w", err)
		}
	}
	return fmt.Errorf("unknown database error: %w", err)
}
