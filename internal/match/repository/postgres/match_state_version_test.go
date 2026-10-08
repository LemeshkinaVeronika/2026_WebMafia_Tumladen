package postgres

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/webmafia/tumladan/internal/model"
)

func TestUpdateStateComparesVersionAsNumber(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	state := model.JSONB(`{"version":2}`)
	mock.ExpectExec(regexp.QuoteMeta("(game_state->>'version')::BIGINT = $5")).
		WithArgs("match-1", state, model.MatchStatusActive, nil, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := New(db)
	if err := repo.UpdateState(t.Context(), "match-1", 1, state, model.MatchStatusActive, nil); err != nil {
		t.Fatalf("UpdateState() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestUpdateStateReturnsConflictWhenVersionChanged(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	state := model.JSONB(`{"version":2}`)
	mock.ExpectExec(regexp.QuoteMeta("(game_state->>'version')::BIGINT = $5")).
		WithArgs("match-1", state, model.MatchStatusActive, nil, 1).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := New(db)
	err = repo.UpdateState(t.Context(), "match-1", 1, state, model.MatchStatusActive, nil)
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("UpdateState() error = %v, want ErrStateConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
