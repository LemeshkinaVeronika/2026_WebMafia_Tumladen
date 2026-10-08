package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/webmafia/tumladan/internal/model"
)

const maxRecentMatchActivities = 50

func (r *Repository) ListRecentActivities(ctx context.Context, matchID string, maxStateVersion, limit int) ([]model.MatchActivity, error) {
	const op = "match.repository.postgres.ListRecentActivities"

	if limit <= 0 || limit > maxRecentMatchActivities {
		limit = maxRecentMatchActivities
	}

	const query = `
		SELECT id, match_id, state_version, ordinal, turn_number, actor_id, type, payload, created_at
		FROM match_activities
		WHERE match_id = $1
		  AND state_version <= $2
		ORDER BY state_version DESC, ordinal DESC
		LIMIT $3
	`
	rows, err := r.db.QueryContext(ctx, query, matchID, maxStateVersion, limit)
	if err != nil {
		return nil, fmt.Errorf("[%s]: query failed: %w", op, err)
	}
	defer rows.Close()

	activities := make([]model.MatchActivity, 0)
	for rows.Next() {
		var activity model.MatchActivity
		if err := rows.Scan(
			&activity.ID,
			&activity.MatchID,
			&activity.StateVersion,
			&activity.Ordinal,
			&activity.TurnNumber,
			&activity.ActorID,
			&activity.Type,
			&activity.Payload,
			&activity.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("[%s]: scan failed: %w", op, err)
		}
		activities = append(activities, activity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("[%s]: rows failed: %w", op, err)
	}

	return activities, nil
}

type activityExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertMatchActivities(ctx context.Context, execer activityExecer, activities []model.MatchActivity) error {
	const query = `
		INSERT INTO match_activities (
			id, match_id, state_version, ordinal, turn_number, actor_id, type, payload, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	for _, activity := range activities {
		payload := activity.Payload
		if len(payload) == 0 {
			payload = model.JSONB(`{}`)
		}
		if _, err := execer.ExecContext(
			ctx,
			query,
			activity.ID,
			activity.MatchID,
			activity.StateVersion,
			activity.Ordinal,
			activity.TurnNumber,
			activity.ActorID,
			activity.Type,
			payload,
			activity.CreatedAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func activitiesForMatch(activities []model.MatchActivity, matchID string) []model.MatchActivity {
	if len(activities) == 0 {
		return nil
	}

	stamped := make([]model.MatchActivity, len(activities))
	for i, activity := range activities {
		activity.MatchID = matchID
		stamped[i] = activity
	}
	return stamped
}
