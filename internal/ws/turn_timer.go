package ws

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	matchDTO "github.com/webmafia/tumladan/internal/match/dto"
	matchService "github.com/webmafia/tumladan/internal/match/service"
	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/pkg/logger"
)

type turnTimerManager struct {
	mu      sync.Mutex
	timers  map[string]*turnTimerEntry
	service *matchService.Service
	logger  logger.Logger
	onApply func(context.Context, string, *matchDTO.MatchResponse)
}

type turnTimerEntry struct {
	timer    *time.Timer
	deadline time.Time
	key      turnTimerKey
}

type turnTimerKey struct {
	MatchID         string
	TurnNumber      int
	CurrentPlayerID string
}

type turnTimerSnapshot struct {
	Version         int    `json:"version"`
	Phase           string `json:"phase"`
	TurnNumber      int    `json:"turnNumber"`
	TurnStartedAt   string `json:"turnStartedAt"`
	CurrentPlayerID string `json:"currentPlayerId"`
	Settings        struct {
		TurnTimeSeconds int `json:"turnTimeSeconds"`
	} `json:"settings"`
}

func newTurnTimerManager(service *matchService.Service, logger logger.Logger) *turnTimerManager {
	return &turnTimerManager{
		timers:  make(map[string]*turnTimerEntry),
		service: service,
		logger:  logger,
	}
}

func (m *turnTimerManager) SetApplyCallback(callback func(context.Context, string, *matchDTO.MatchResponse)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onApply = callback
}

func (m *turnTimerManager) Schedule(matchState *matchDTO.MatchResponse) {
	if m == nil || matchState == nil {
		return
	}

	snapshot, ok := timeoutSnapshot(matchState)
	if !ok || snapshot.Settings.TurnTimeSeconds == 0 {
		m.Stop(matchState.RoomID)
		return
	}

	key := turnTimerKey{
		MatchID:         matchState.ID,
		TurnNumber:      snapshot.TurnNumber,
		CurrentPlayerID: snapshot.CurrentPlayerID,
	}
	deadline := turnDeadline(turnStartedAt(matchState.UpdatedAt, snapshot), snapshot.Settings.TurnTimeSeconds)
	req := matchDTO.ApplyTurnTimeoutRequest{
		RoomID:               matchState.RoomID,
		ExpectedMatchID:      matchState.ID,
		ExpectedActorID:      snapshot.CurrentPlayerID,
		ExpectedPhase:        snapshot.Phase,
		ExpectedTurnNumber:   snapshot.TurnNumber,
		ExpectedStateVersion: snapshot.Version,
	}

	m.mu.Lock()
	if existing := m.timers[matchState.RoomID]; existing != nil {
		if existing.key == key {
			deadline = existing.deadline
		}
		existing.timer.Stop()
	}
	duration := time.Until(deadline)
	if duration < 0 {
		duration = 0
	}
	m.timers[matchState.RoomID] = &turnTimerEntry{
		deadline: deadline,
		key:      key,
	}
	m.timers[matchState.RoomID].timer = time.AfterFunc(duration, func() {
		m.applyTimeout(req)
	})
	m.mu.Unlock()
}

func (m *turnTimerManager) Stop(roomID string) {
	if m == nil || roomID == "" {
		return
	}

	m.mu.Lock()
	if entry := m.timers[roomID]; entry != nil {
		entry.timer.Stop()
	}
	delete(m.timers, roomID)
	m.mu.Unlock()
}

func (m *turnTimerManager) StopAll() {
	if m == nil {
		return
	}

	m.mu.Lock()
	for roomID, entry := range m.timers {
		entry.timer.Stop()
		delete(m.timers, roomID)
	}
	m.mu.Unlock()
}

func (m *turnTimerManager) applyTimeout(req matchDTO.ApplyTurnTimeoutRequest) {
	ctx := context.Background()
	matchState, err := m.service.ApplyTurnTimeout(ctx, req)
	if err != nil {
		if errors.Is(err, matchService.ErrInvalidMatchAction) ||
			errors.Is(err, matchService.ErrMatchNotFound) ||
			errors.Is(err, matchService.ErrMatchNotActive) {
			return
		}
		m.logger.With("roomID", req.RoomID, "error", err).Warnf("apply turn timeout failed")
		return
	}

	m.mu.Lock()
	callback := m.onApply
	m.mu.Unlock()

	if callback != nil {
		callback(ctx, req.RoomID, matchState)
	}
}

func timeoutSnapshot(matchState *matchDTO.MatchResponse) (turnTimerSnapshot, bool) {
	var snapshot turnTimerSnapshot
	if matchState.Status != string(model.MatchStatusActive) {
		return snapshot, false
	}
	if err := json.Unmarshal(matchState.GameState, &snapshot); err != nil {
		return snapshot, false
	}
	if snapshot.CurrentPlayerID == "" {
		return snapshot, false
	}
	if snapshot.Phase != "place_tile" && snapshot.Phase != "place_meeple" {
		return snapshot, false
	}
	return snapshot, true
}

func turnDeadline(updatedAt string, turnTimeSeconds int) time.Time {
	duration := time.Duration(turnTimeSeconds) * time.Second
	updated, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil || updated.IsZero() {
		return time.Now().Add(duration)
	}
	return updated.Add(duration)
}

func turnStartedAt(fallback string, snapshot turnTimerSnapshot) string {
	if snapshot.TurnStartedAt != "" {
		return snapshot.TurnStartedAt
	}
	return fallback
}
