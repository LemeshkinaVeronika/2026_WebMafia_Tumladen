package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/webmafia/tumladan/internal/event"
	"github.com/webmafia/tumladan/internal/model"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ApplyMatchFinished(
	ctx context.Context,
	consumerGroup string,
	eventID string,
	match event.MatchFinished,
) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin projection transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var inserted int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO processed_events (consumer_group, event_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
		RETURNING 1
	`, consumerGroup, eventID).Scan(&inserted)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return false, fmt.Errorf("commit duplicate event: %w", err)
		}
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("deduplicate event: %w", err)
	}

	if match.TerminationReason == model.MatchTerminationReasonNormalCompletion {
		for _, player := range match.Players {
			if player.ActorType != model.ActorTypeUser {
				continue
			}
			wins, losses, draws := 0, 0, 0
			switch {
			case player.IsDraw:
				draws = 1
			case player.IsWinner:
				wins = 1
			default:
				losses = 1
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO user_game_stats (
					user_id, game_type, matches, wins, losses, draws, total_score, best_score
				) VALUES ($1, $2, 1, $3, $4, $5, $6, $7)
				ON CONFLICT (user_id, game_type) DO UPDATE SET
					matches = user_game_stats.matches + 1,
					wins = user_game_stats.wins + EXCLUDED.wins,
					losses = user_game_stats.losses + EXCLUDED.losses,
					draws = user_game_stats.draws + EXCLUDED.draws,
					total_score = user_game_stats.total_score + EXCLUDED.total_score,
					best_score = GREATEST(user_game_stats.best_score, EXCLUDED.best_score),
					updated_at = NOW()
			`, player.ActorID, match.GameType, wins, losses, draws, player.Score, player.Score); err != nil {
				return false, fmt.Errorf("update stats for user %s: %w", player.ActorID, err)
			}

			achievements := []string{model.AchievementFirstGameAny}
			if match.GameType == "carcassonne" {
				achievements = append(achievements, model.AchievementCarcassonneFirstGame)
				if player.IsWinner {
					achievements = append(achievements, model.AchievementCarcassonneFirstWin)
				}
			}
			for _, code := range achievements {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO user_achievements (user_id, achievement_code, match_id, unlocked_at)
					VALUES ($1, $2, $3, $4)
					ON CONFLICT (user_id, achievement_code) DO NOTHING
				`, player.ActorID, code, match.MatchID, match.TerminatedAt); err != nil {
					return false, fmt.Errorf("unlock achievement %s: %w", code, err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit match projection: %w", err)
	}
	return true, nil
}
