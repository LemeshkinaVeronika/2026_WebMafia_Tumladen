package postgres

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/webmafia/tumladan/internal/model"
)

func TestPersistActionResultCommitsStateReceiptAndActivitiesTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	receipt := testActionReceipt()
	state := model.JSONB(`{"version":2}`)
	createdAt := time.Date(2026, time.October, 8, 12, 0, 0, 0, time.UTC)
	activity := model.MatchActivity{
		ID:           "00000000-0000-0000-0000-000000000003",
		MatchID:      "untrusted-match-id",
		StateVersion: 2,
		Ordinal:      0,
		TurnNumber:   1,
		ActorID:      receipt.ActorID,
		Type:         "tile_placed",
		Payload:      model.JSONB(`{"tileId":"city_cap"}`),
		CreatedAt:    createdAt,
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_action_receipts")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("(game_state->>'version')::BIGINT = $5")).
		WithArgs(receipt.MatchID, state, model.MatchStatusActive, nil, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_activities")).
		WithArgs(
			activity.ID,
			receipt.MatchID,
			activity.StateVersion,
			activity.Ordinal,
			activity.TurnNumber,
			activity.ActorID,
			activity.Type,
			activity.Payload,
			activity.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := New(db)
	err = repo.PersistActionResult(
		t.Context(),
		receipt.MatchID,
		1,
		state,
		model.MatchStatusActive,
		nil,
		receipt,
		[]model.MatchActivity{activity},
	)
	if err != nil {
		t.Fatalf("PersistActionResult() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestPersistActionResultRollsBackWhenActivityInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	insertErr := errors.New("activity insert failed")
	matchID := "00000000-0000-0000-0000-000000000002"
	state := model.JSONB(`{"version":2}`)
	activity := model.MatchActivity{
		ID:           "00000000-0000-0000-0000-000000000003",
		StateVersion: 2,
		TurnNumber:   1,
		ActorID:      "actor-1",
		Type:         "tile_placed",
		Payload:      model.JSONB(`{}`),
		CreatedAt:    time.Now().UTC(),
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("(game_state->>'version')::BIGINT = $5")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_activities")).
		WillReturnError(insertErr)
	mock.ExpectRollback()

	repo := New(db)
	err = repo.PersistActionResult(
		t.Context(),
		matchID,
		1,
		state,
		model.MatchStatusActive,
		nil,
		nil,
		[]model.MatchActivity{activity},
	)
	if !errors.Is(err, insertErr) {
		t.Fatalf("PersistActionResult() error = %v, want activity insert error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestListRecentActivitiesReturnsNewestFirstWithRawPayload(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	matchID := "00000000-0000-0000-0000-000000000002"
	createdAt := time.Date(2026, time.October, 8, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("AND state_version <= $2")).
		WithArgs(matchID, 3, 50).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "match_id", "state_version", "ordinal", "turn_number", "actor_id", "type", "payload", "created_at",
		}).
			AddRow("activity-2", matchID, 3, 0, 2, "actor-2", "meeple_placed", []byte(`{"featureType":"road"}`), createdAt.Add(time.Second)).
			AddRow("activity-1", matchID, 2, 0, 1, "actor-1", "tile_placed", []byte(`{"tileId":"city_cap"}`), createdAt))

	activities, err := New(db).ListRecentActivities(t.Context(), matchID, 3, 50)
	if err != nil {
		t.Fatalf("ListRecentActivities() error = %v", err)
	}
	if len(activities) != 2 || activities[0].ID != "activity-2" || activities[1].ID != "activity-1" {
		t.Fatalf("activities = %#v, want newest first", activities)
	}
	if got, want := string(activities[0].Payload), `{"featureType":"road"}`; got != want {
		t.Fatalf("newest payload = %q, want %q", got, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
