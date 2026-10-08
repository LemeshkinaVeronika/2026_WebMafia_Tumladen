package postgres

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/webmafia/tumladan/internal/model"
)

func TestUpdateStateWithActionReceiptCommitsReceiptAndStateTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	receipt := testActionReceipt()
	state := model.JSONB(`{"version":2}`)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_action_receipts")).
		WithArgs(
			receipt.RoomID,
			receipt.MatchID,
			receipt.ActorID,
			receipt.ActionID,
			receipt.RequestHash,
			receipt.Status,
			receipt.StateVersion,
			receipt.ErrorCode,
			receipt.ErrorMessage,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("(game_state->>'version')::BIGINT = $5")).
		WithArgs(receipt.MatchID, state, model.MatchStatusActive, nil, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := New(db)
	if err := repo.UpdateStateWithActionReceipt(t.Context(), receipt.MatchID, 1, state, model.MatchStatusActive, nil, receipt); err != nil {
		t.Fatalf("UpdateStateWithActionReceipt() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestUpdateStateWithActionReceiptRollsBackWhenReceiptInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	receipt := testActionReceipt()
	insertErr := errors.New("receipt insert failed")
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_action_receipts")).
		WithArgs(
			receipt.RoomID,
			receipt.MatchID,
			receipt.ActorID,
			receipt.ActionID,
			receipt.RequestHash,
			receipt.Status,
			receipt.StateVersion,
			receipt.ErrorCode,
			receipt.ErrorMessage,
		).
		WillReturnError(insertErr)
	mock.ExpectRollback()

	repo := New(db)
	err = repo.UpdateStateWithActionReceipt(t.Context(), receipt.MatchID, 1, model.JSONB(`{"version":2}`), model.MatchStatusActive, nil, receipt)
	if !errors.Is(err, insertErr) {
		t.Fatalf("UpdateStateWithActionReceipt() error = %v, want insert error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestUpdateStateWithActionReceiptRollsBackWhenMatchUpdateDoesNotApply(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	receipt := testActionReceipt()
	state := model.JSONB(`{"version":2}`)
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_action_receipts")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("(game_state->>'version')::BIGINT = $5")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	repo := New(db)
	err = repo.UpdateStateWithActionReceipt(t.Context(), receipt.MatchID, 1, state, model.MatchStatusActive, nil, receipt)
	if !errors.Is(err, ErrStateConflict) {
		t.Fatalf("UpdateStateWithActionReceipt() error = %v, want ErrStateConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGetActionReceiptUsesActorScopedIdempotencyKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	receipt := testActionReceipt()
	mock.ExpectQuery(regexp.QuoteMeta("FROM match_action_receipts")).
		WithArgs(receipt.RoomID, receipt.ActorID, receipt.ActionID).
		WillReturnRows(sqlmock.NewRows([]string{
			"room_id", "match_id", "actor_id", "action_id", "request_hash", "status",
			"state_version", "error_code", "error_message", "created_at",
		}).AddRow(
			receipt.RoomID,
			receipt.MatchID,
			receipt.ActorID,
			receipt.ActionID,
			receipt.RequestHash,
			receipt.Status,
			receipt.StateVersion,
			"",
			"",
			receipt.CreatedAt,
		))

	repo := New(db)
	got, err := repo.GetActionReceipt(t.Context(), receipt.RoomID, receipt.ActorID, receipt.ActionID)
	if err != nil {
		t.Fatalf("GetActionReceipt() error = %v", err)
	}
	if *got != *receipt {
		t.Fatalf("GetActionReceipt() = %#v, want %#v", got, receipt)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func testActionReceipt() *model.MatchActionReceipt {
	return &model.MatchActionReceipt{
		RoomID:       "00000000-0000-0000-0000-000000000001",
		MatchID:      "00000000-0000-0000-0000-000000000002",
		ActorID:      "actor-1",
		ActionID:     "action-1",
		RequestHash:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Status:       model.MatchActionStatusAccepted,
		StateVersion: 2,
		CreatedAt:    testReceiptTime,
	}
}

var testReceiptTime = time.Date(2026, time.October, 7, 12, 0, 0, 0, time.UTC)
