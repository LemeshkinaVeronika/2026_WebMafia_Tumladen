package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/webmafia/tumladan/internal/model"
)

func TestNewMatchFinishedBuildsVersionedEvent(t *testing.T) {
	result := model.JSONB(`{"winners":["user-1","user-2"],"finalScores":[{"actorId":"user-1","score":42},{"actorId":"user-2","score":42}]}`)
	occurredAt := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	envelope, err := NewMatchFinished(
		&model.Match{ID: "match-1", RoomID: "room-1", GameType: "carcassonne"},
		[]model.MatchPlayer{
			{ActorID: "user-1", ActorType: model.ActorTypeUser, DisplayName: "One", Seat: 0},
			{ActorID: "user-2", ActorType: model.ActorTypeUser, DisplayName: "Two", Seat: 1},
		},
		&result,
		model.MatchTerminationReasonNormalCompletion,
		occurredAt,
	)
	if err != nil {
		t.Fatalf("NewMatchFinished() error = %v", err)
	}
	if envelope.ID == "" || envelope.Type != MatchFinishedType || envelope.Version != 1 {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}

	var payload MatchFinished
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.MatchID != "match-1" || payload.TerminationReason != model.MatchTerminationReasonNormalCompletion {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if len(payload.Players) != 2 || !payload.Players[0].IsDraw || payload.Players[0].Score != 42 {
		t.Fatalf("unexpected player result: %+v", payload.Players)
	}
}
