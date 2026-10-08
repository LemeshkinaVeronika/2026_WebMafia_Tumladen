package ws

import (
	"encoding/json"
	"testing"
	"time"

	matchDTO "github.com/webmafia/tumladan/internal/match/dto"
	"github.com/webmafia/tumladan/internal/model"
)

func TestTurnTimerScheduleIgnoresOlderVersionOfSameMatch(t *testing.T) {
	manager := &turnTimerManager{timers: make(map[string]*turnTimerEntry)}
	defer manager.StopAll()

	manager.Schedule(testTimerMatch("match-1", model.MatchStatusActive, 3, "place_meeple"))
	current := manager.timers["room-1"]
	if current == nil || current.version != 3 {
		t.Fatalf("current timer = %#v, want version 3", current)
	}

	manager.Schedule(testTimerMatch("match-1", model.MatchStatusActive, 2, "place_tile"))
	if got := manager.timers["room-1"]; got != current {
		t.Fatalf("older snapshot replaced current timer: got %#v, want %#v", got, current)
	}
}

func TestTurnTimerScheduleUpdatesSameTurnWithoutResettingDeadline(t *testing.T) {
	manager := &turnTimerManager{timers: make(map[string]*turnTimerEntry)}
	defer manager.StopAll()

	manager.Schedule(testTimerMatch("match-1", model.MatchStatusActive, 1, "place_tile"))
	first := manager.timers["room-1"]
	if first == nil {
		t.Fatal("first timer was not scheduled")
	}

	manager.Schedule(testTimerMatch("match-1", model.MatchStatusActive, 2, "place_meeple"))
	second := manager.timers["room-1"]
	if second == nil || second.version != 2 {
		t.Fatalf("updated timer = %#v, want version 2", second)
	}
	if !second.deadline.Equal(first.deadline) {
		t.Fatalf("same turn deadline changed from %s to %s", first.deadline, second.deadline)
	}
}

func TestTurnTimerStaleFinishedMatchDoesNotStopNewMatchTimer(t *testing.T) {
	manager := &turnTimerManager{timers: make(map[string]*turnTimerEntry)}
	defer manager.StopAll()

	manager.Schedule(testTimerMatch("match-2", model.MatchStatusActive, 1, "place_tile"))
	current := manager.timers["room-1"]
	manager.Schedule(testTimerMatch("match-1", model.MatchStatusFinished, 8, "finished"))

	if got := manager.timers["room-1"]; got != current {
		t.Fatalf("historical finished match stopped current timer: got %#v, want %#v", got, current)
	}
}

func TestTurnTimerOlderActiveMatchDoesNotReplaceNewMatchTimer(t *testing.T) {
	manager := &turnTimerManager{timers: make(map[string]*turnTimerEntry)}
	defer manager.StopAll()

	newMatch := testTimerMatch("match-2", model.MatchStatusActive, 1, "place_tile")
	newMatch.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	oldMatch := testTimerMatch("match-1", model.MatchStatusActive, 8, "place_meeple")
	oldMatch.CreatedAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano)

	manager.Schedule(newMatch)
	current := manager.timers["room-1"]
	manager.Schedule(oldMatch)

	if got := manager.timers["room-1"]; got != current {
		t.Fatalf("older active match replaced current timer: got %#v, want %#v", got, current)
	}
}

func testTimerMatch(matchID string, status model.MatchStatus, version int, phase string) *matchDTO.MatchResponse {
	startedAt := time.Now().UTC().Format(time.RFC3339)
	state, err := json.Marshal(map[string]any{
		"version":         version,
		"phase":           phase,
		"turnNumber":      4,
		"turnStartedAt":   startedAt,
		"currentPlayerId": "actor-1",
		"settings": map[string]any{
			"turnTimeSeconds": 3600,
		},
	})
	if err != nil {
		panic(err)
	}
	return &matchDTO.MatchResponse{
		ID:        matchID,
		RoomID:    "room-1",
		Status:    string(status),
		GameState: state,
		CreatedAt: startedAt,
		UpdatedAt: startedAt,
	}
}
