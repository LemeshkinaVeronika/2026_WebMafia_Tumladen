package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/webmafia/tumladan/internal/event"
	"github.com/webmafia/tumladan/internal/model"
)

func TestApplyMatchFinishedUpdatesProjectionAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	terminatedAt := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO processed_events").
		WithArgs("stats-v1", "50e71ca7-f4ad-4e88-a7bc-48e08e740c4e").
		WillReturnRows(sqlmock.NewRows([]string{"inserted"}).AddRow(1))
	mock.ExpectExec("INSERT INTO user_game_stats").
		WithArgs("70e71ca7-f4ad-4e88-a7bc-48e08e740c4e", "carcassonne", 1, 0, 0, 42, 42).
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, code := range []string{
		model.AchievementFirstGameAny,
		model.AchievementCarcassonneFirstGame,
		model.AchievementCarcassonneFirstWin,
	} {
		mock.ExpectExec("INSERT INTO user_achievements").
			WithArgs("70e71ca7-f4ad-4e88-a7bc-48e08e740c4e", code, "match-1", terminatedAt).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	applied, err := New(db).ApplyMatchFinished(context.Background(), "stats-v1", "50e71ca7-f4ad-4e88-a7bc-48e08e740c4e", event.MatchFinished{
		MatchID:           "match-1",
		GameType:          "carcassonne",
		TerminationReason: model.MatchTerminationReasonNormalCompletion,
		TerminatedAt:      terminatedAt,
		Players: []event.MatchFinishedPlayer{{
			ActorID:   "70e71ca7-f4ad-4e88-a7bc-48e08e740c4e",
			ActorType: model.ActorTypeUser,
			Score:     42,
			IsWinner:  true,
		}},
	})
	if err != nil || !applied {
		t.Fatalf("ApplyMatchFinished() = (%v, %v), want (true, nil)", applied, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyMatchFinishedSkipsDuplicateEvent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO processed_events").
		WithArgs("stats-v1", "50e71ca7-f4ad-4e88-a7bc-48e08e740c4e").
		WillReturnRows(sqlmock.NewRows([]string{"inserted"}))
	mock.ExpectCommit()

	applied, err := New(db).ApplyMatchFinished(
		context.Background(),
		"stats-v1",
		"50e71ca7-f4ad-4e88-a7bc-48e08e740c4e",
		event.MatchFinished{},
	)
	if err != nil || applied {
		t.Fatalf("ApplyMatchFinished() = (%v, %v), want (false, nil)", applied, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
