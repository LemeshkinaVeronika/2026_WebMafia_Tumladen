package postgres

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/webmafia/tumladan/internal/model"
)

func TestTerminateActiveMatchCommitsReceiptStateOutboxAndRoomTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	fixture := terminalActionFixture()
	expectTerminalActionLoad(mock, fixture)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_action_receipts")).
		WithArgs(
			fixture.receipt.RoomID,
			fixture.matchID,
			fixture.receipt.ActorID,
			fixture.receipt.ActionID,
			fixture.receipt.RequestHash,
			fixture.receipt.Status,
			fixture.receipt.StateVersion,
			fixture.receipt.ErrorCode,
			fixture.receipt.ErrorMessage,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE matches")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO outbox_events")).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE rooms")).
		WithArgs(fixture.roomID, model.RoomStatusWaiting).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(fixture.terminatedAt))
	mock.ExpectCommit()

	repo := New(db)
	match, _, err := repo.TerminateActiveMatch(
		t.Context(),
		fixture.roomID,
		&fixture.nextState,
		model.MatchTerminationReasonNormalCompletion,
		nil,
		&fixture.actorID,
		fixture.terminatedAt,
		&fixture.expectedVersion,
		fixture.receipt,
	)
	if err != nil {
		t.Fatalf("TerminateActiveMatch() error = %v", err)
	}
	if match.Status != model.MatchStatusFinished || string(match.GameState) != string(fixture.nextState) {
		t.Fatalf("terminated match = %#v, want finished with next state", match)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestTerminateActiveMatchRollsBackEverythingWhenReceiptInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	fixture := terminalActionFixture()
	insertErr := errors.New("receipt insert failed")
	expectTerminalActionLoad(mock, fixture)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO match_action_receipts")).WillReturnError(insertErr)
	mock.ExpectRollback()

	repo := New(db)
	_, _, err = repo.TerminateActiveMatch(
		t.Context(),
		fixture.roomID,
		&fixture.nextState,
		model.MatchTerminationReasonNormalCompletion,
		nil,
		&fixture.actorID,
		fixture.terminatedAt,
		&fixture.expectedVersion,
		fixture.receipt,
	)
	if !errors.Is(err, insertErr) {
		t.Fatalf("TerminateActiveMatch() error = %v, want receipt insert error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestTerminateActiveMatchRejectsStaleVersionBeforeWriting(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	fixture := terminalActionFixture()
	fixture.expectedVersion = 0
	mock.ExpectBegin()
	expectRoomForUpdate(mock, fixture)
	expectMatchForUpdate(mock, fixture)
	mock.ExpectRollback()

	repo := New(db)
	_, _, err = repo.TerminateActiveMatch(
		t.Context(),
		fixture.roomID,
		&fixture.nextState,
		model.MatchTerminationReasonNormalCompletion,
		nil,
		&fixture.actorID,
		fixture.terminatedAt,
		&fixture.expectedVersion,
		fixture.receipt,
	)
	if !errors.Is(err, ErrMatchStateConflict) {
		t.Fatalf("TerminateActiveMatch() error = %v, want ErrMatchStateConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

type terminalFixture struct {
	roomID          string
	matchID         string
	actorID         string
	createdAt       time.Time
	updatedAt       time.Time
	terminatedAt    time.Time
	expectedVersion int
	nextState       model.JSONB
	receipt         *model.MatchActionReceipt
}

func terminalActionFixture() terminalFixture {
	createdAt := time.Date(2026, time.October, 7, 10, 0, 0, 0, time.UTC)
	return terminalFixture{
		roomID:          "00000000-0000-0000-0000-000000000001",
		matchID:         "00000000-0000-0000-0000-000000000002",
		actorID:         "actor-1",
		createdAt:       createdAt,
		updatedAt:       createdAt.Add(time.Minute),
		terminatedAt:    createdAt.Add(2 * time.Minute),
		expectedVersion: 1,
		nextState:       model.JSONB(`{"version":2,"phase":"finished"}`),
		receipt: &model.MatchActionReceipt{
			RoomID:       "00000000-0000-0000-0000-000000000001",
			MatchID:      "00000000-0000-0000-0000-000000000002",
			ActorID:      "actor-1",
			ActionID:     "action-1",
			RequestHash:  "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			Status:       model.MatchActionStatusAccepted,
			StateVersion: 2,
		},
	}
}

func expectTerminalActionLoad(mock sqlmock.Sqlmock, fixture terminalFixture) {
	mock.ExpectBegin()
	expectRoomForUpdate(mock, fixture)
	expectMatchForUpdate(mock, fixture)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT mp.match_id")).
		WithArgs(fixture.matchID).
		WillReturnRows(sqlmock.NewRows([]string{
			"match_id", "actor_id", "actor_type", "display_name", "bot_difficulty", "avatar_url", "seat", "disconnected_at",
		}))
}

func expectRoomForUpdate(mock sqlmock.Sqlmock, fixture terminalFixture) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, is_private")).
		WithArgs(fixture.roomID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "is_private", "invite_code", "owner_actor_id", "owner_actor_type", "status",
			"game_type", "max_players", "settings", "last_empty_at", "created_at", "updated_at",
		}).AddRow(
			fixture.roomID,
			"Room",
			false,
			nil,
			fixture.actorID,
			model.ActorTypeUser,
			model.RoomStatusPlaying,
			"carcassonne",
			2,
			[]byte(`{}`),
			nil,
			fixture.createdAt,
			fixture.updatedAt,
		))
}

func expectMatchForUpdate(mock sqlmock.Sqlmock, fixture terminalFixture) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, room_id, game_type, status, game_state")).
		WithArgs(fixture.roomID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "room_id", "game_type", "status", "game_state", "result", "termination_reason",
			"terminated_by_actor_id", "terminated_at", "created_at", "updated_at",
		}).AddRow(
			fixture.matchID,
			fixture.roomID,
			"carcassonne",
			model.MatchStatusActive,
			[]byte(`{"version":1,"phase":"place_meeple"}`),
			nil,
			nil,
			nil,
			nil,
			fixture.createdAt,
			fixture.updatedAt,
		))
}
