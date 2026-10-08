package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gameService "github.com/webmafia/tumladan/internal/game/service"
	matchDTO "github.com/webmafia/tumladan/internal/match/dto"
	matchPostgres "github.com/webmafia/tumladan/internal/match/repository/postgres"
	"github.com/webmafia/tumladan/internal/model"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
)

func TestApplyActionPersistsOnceAndReplaysAcceptedReceipt(t *testing.T) {
	match := testReliableMatch(model.MatchStatusActive, `{"version":1,"phase":"place_tile","turnNumber":1,"currentPlayerId":"actor-1"}`)
	repo := &reliableActionRepo{match: match}
	engine := &reliableActionEngine{
		result: gameService.ApplyActionResult{
			NextState:  model.JSONB(`{"version":1,"phase":"waiting","turnNumber":1,"currentPlayerId":"actor-1"}`),
			NextStatus: model.MatchStatusActive,
		},
	}
	registry, err := gameService.NewRegistry(engine)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	service := New(repo, nil, gameService.NewFacade(registry))
	req := testReliableActionRequest()

	first, err := service.ApplyAction(t.Context(), req)
	if err != nil {
		t.Fatalf("first ApplyAction() error = %v", err)
	}
	if first.Replayed || first.Status != string(model.MatchActionStatusAccepted) || first.StateVersion != 2 {
		t.Fatalf("first result = %#v, want newly accepted version 2", first)
	}
	if engine.applyCalls != 1 || repo.updateWithReceiptCalls != 1 {
		t.Fatalf("after first action engine calls = %d, atomic updates = %d; want 1, 1", engine.applyCalls, repo.updateWithReceiptCalls)
	}
	if repo.listActivitiesVersion != 2 {
		t.Fatalf("activity snapshot version = %d, want 2", repo.listActivitiesVersion)
	}

	second, err := service.ApplyAction(t.Context(), req)
	if err != nil {
		t.Fatalf("replayed ApplyAction() error = %v", err)
	}
	if !second.Replayed || second.Status != string(model.MatchActionStatusAccepted) || second.StateVersion != 2 {
		t.Fatalf("replayed result = %#v, want accepted replay of version 2", second)
	}
	if engine.applyCalls != 1 || repo.updateWithReceiptCalls != 1 {
		t.Fatalf("after replay engine calls = %d, atomic updates = %d; want unchanged 1, 1", engine.applyCalls, repo.updateWithReceiptCalls)
	}
	if got := gameStateVersion(repo.match.GameState); got != 2 {
		t.Fatalf("persisted version = %d, want 2", got)
	}
}

func TestApplyActionReturnsCommittedStateWhenFollowingBotStepFails(t *testing.T) {
	match := testReliableMatch(model.MatchStatusActive, `{"version":1,"phase":"place_tile","turnNumber":1,"currentPlayerId":"actor-1"}`)
	repo := &reliableActionRepo{
		match: match,
		players: []model.MatchPlayer{
			{MatchID: match.ID, ActorID: "actor-1", ActorType: model.ActorTypeUser, Seat: 0},
			{MatchID: match.ID, ActorID: "bot-1", ActorType: model.ActorTypeBot, Seat: 1},
		},
	}
	botErr := errors.New("build bot action failed")
	engine := &reliableActionEngine{
		result: gameService.ApplyActionResult{
			NextState:  model.JSONB(`{"version":1,"phase":"place_tile","turnNumber":2,"currentPlayerId":"bot-1"}`),
			NextStatus: model.MatchStatusActive,
		},
		botBuildErr: botErr,
	}
	registry, err := gameService.NewRegistry(engine)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	service := New(repo, nil, gameService.NewFacade(registry))

	result, err := service.ApplyAction(t.Context(), testReliableActionRequest())
	if !errors.Is(err, botErr) {
		t.Fatalf("ApplyAction() error = %v, want bot error", err)
	}
	if result == nil || result.Status != string(model.MatchActionStatusAccepted) || result.MatchState == nil {
		t.Fatalf("result = %#v, want accepted action with committed snapshot", result)
	}
	if got := gameStateVersion(result.MatchState.GameState); got != 2 {
		t.Fatalf("returned state version = %d, want committed version 2", got)
	}
	if repo.receipt == nil || repo.updateWithReceiptCalls != 1 {
		t.Fatalf("receipt = %#v, atomic updates = %d; want committed receipt", repo.receipt, repo.updateWithReceiptCalls)
	}
}

func TestApplyActionReplaysTerminalReceiptWhenThereIsNoNewActiveMatch(t *testing.T) {
	req := testReliableActionRequest()
	hash, err := matchActionRequestHash(req)
	if err != nil {
		t.Fatalf("matchActionRequestHash() error = %v", err)
	}
	match := testReliableMatch(model.MatchStatusFinished, `{"version":7,"phase":"finished","turnNumber":4}`)
	repo := &reliableActionRepo{
		match: match,
		receipt: &model.MatchActionReceipt{
			RoomID:       req.RoomID,
			MatchID:      match.ID,
			ActorID:      req.ActorID,
			ActionID:     req.ActionID,
			RequestHash:  hash,
			Status:       model.MatchActionStatusAccepted,
			StateVersion: 6,
		},
	}
	service := New(repo, nil, nil)

	result, err := service.ApplyAction(t.Context(), req)
	if err != nil {
		t.Fatalf("ApplyAction() error = %v", err)
	}
	if !result.Replayed || result.StateVersion != 6 || result.MatchState.Status != string(model.MatchStatusFinished) {
		t.Fatalf("result = %#v, want accepted terminal replay at original version 6", result)
	}
	if repo.getActiveCalls != 1 {
		t.Fatalf("GetActiveByRoomID calls = %d, want 1", repo.getActiveCalls)
	}
}

func TestApplyActionReplayReturnsNewActiveMatchInsteadOfHistoricalMatch(t *testing.T) {
	req := testReliableActionRequest()
	hash, err := matchActionRequestHash(req)
	if err != nil {
		t.Fatalf("matchActionRequestHash() error = %v", err)
	}
	historicalMatch := testReliableMatch(model.MatchStatusFinished, `{"version":7,"phase":"finished","turnNumber":4}`)
	activeMatch := testReliableMatch(model.MatchStatusActive, `{"version":1,"phase":"place_tile","turnNumber":1,"currentPlayerId":"actor-1"}`)
	activeMatch.ID = "match-2"
	repo := &reliableActionRepo{
		match:       historicalMatch,
		activeMatch: activeMatch,
		players: []model.MatchPlayer{
			{MatchID: activeMatch.ID, ActorID: req.ActorID, ActorType: model.ActorTypeUser},
		},
		receipt: &model.MatchActionReceipt{
			RoomID:       req.RoomID,
			MatchID:      historicalMatch.ID,
			ActorID:      req.ActorID,
			ActionID:     req.ActionID,
			RequestHash:  hash,
			Status:       model.MatchActionStatusAccepted,
			StateVersion: 6,
		},
	}
	service := New(repo, nil, nil)

	result, err := service.ApplyAction(t.Context(), req)
	if err != nil {
		t.Fatalf("ApplyAction() error = %v", err)
	}
	if !result.Replayed || result.StateVersion != 6 || result.MatchState == nil || result.MatchState.ID != activeMatch.ID {
		t.Fatalf("result = %#v, want receipt replay reconciled with active match %q", result, activeMatch.ID)
	}
}

func TestApplyActionReplayDoesNotRevealNewMatchToFormerPlayer(t *testing.T) {
	req := testReliableActionRequest()
	hash, err := matchActionRequestHash(req)
	if err != nil {
		t.Fatalf("matchActionRequestHash() error = %v", err)
	}
	historicalMatch := testReliableMatch(model.MatchStatusFinished, `{"version":7,"phase":"finished","turnNumber":4}`)
	activeMatch := testReliableMatch(model.MatchStatusActive, `{"version":1,"phase":"place_tile","turnNumber":1,"currentPlayerId":"actor-2"}`)
	activeMatch.ID = "match-2"
	repo := &reliableActionRepo{
		match:       historicalMatch,
		activeMatch: activeMatch,
		players: []model.MatchPlayer{
			{MatchID: activeMatch.ID, ActorID: "actor-2", ActorType: model.ActorTypeUser},
		},
		receipt: &model.MatchActionReceipt{
			RoomID:       req.RoomID,
			MatchID:      historicalMatch.ID,
			ActorID:      req.ActorID,
			ActionID:     req.ActionID,
			RequestHash:  hash,
			Status:       model.MatchActionStatusAccepted,
			StateVersion: 6,
		},
	}
	service := New(repo, nil, nil)

	result, err := service.ApplyAction(t.Context(), req)
	if err != nil {
		t.Fatalf("ApplyAction() error = %v", err)
	}
	if result.MatchState == nil || result.MatchState.ID != historicalMatch.ID {
		t.Fatalf("result state = %#v, want historical match %q", result.MatchState, historicalMatch.ID)
	}
}

func TestApplyActionPersistsAndReplaysRejectedStateConflict(t *testing.T) {
	match := testReliableMatch(model.MatchStatusActive, `{"version":3,"phase":"place_meeple","turnNumber":2,"currentPlayerId":"actor-1"}`)
	repo := &reliableActionRepo{match: match}
	service := New(repo, nil, nil)
	req := testReliableActionRequest()

	first, err := service.ApplyAction(t.Context(), req)
	if !errors.Is(err, ErrMatchStateConflict) {
		t.Fatalf("first ApplyAction() error = %v, want ErrMatchStateConflict", err)
	}
	if first == nil || first.Replayed || first.StateVersion != 3 || first.ErrorCode != "MATCH_STATE_CONFLICT" {
		t.Fatalf("first result = %#v, want persisted state conflict at version 3", first)
	}
	if repo.saveReceiptCalls != 1 || repo.receipt == nil {
		t.Fatalf("SaveActionReceipt calls = %d, receipt = %#v", repo.saveReceiptCalls, repo.receipt)
	}

	second, err := service.ApplyAction(t.Context(), req)
	if !errors.Is(err, ErrMatchStateConflict) {
		t.Fatalf("replayed ApplyAction() error = %v, want ErrMatchStateConflict", err)
	}
	if second == nil || !second.Replayed || second.StateVersion != 3 || second.ErrorCode != first.ErrorCode || second.ErrorMessage != first.ErrorMessage {
		t.Fatalf("replayed result = %#v, want original rejected receipt %#v", second, first)
	}
	if repo.getActiveCalls != 2 || repo.saveReceiptCalls != 1 {
		t.Fatalf("after replay active lookups = %d, receipt saves = %d; want 2, 1", repo.getActiveCalls, repo.saveReceiptCalls)
	}
}

func TestApplyActionRejectsReusedActionIDWithDifferentRequest(t *testing.T) {
	req := testReliableActionRequest()
	hash, err := matchActionRequestHash(req)
	if err != nil {
		t.Fatalf("matchActionRequestHash() error = %v", err)
	}
	match := testReliableMatch(model.MatchStatusActive, `{"version":2,"phase":"place_meeple","turnNumber":1}`)
	repo := &reliableActionRepo{
		match: match,
		receipt: &model.MatchActionReceipt{
			RoomID:       req.RoomID,
			MatchID:      match.ID,
			ActorID:      req.ActorID,
			ActionID:     req.ActionID,
			RequestHash:  hash,
			Status:       model.MatchActionStatusAccepted,
			StateVersion: 2,
		},
	}
	service := New(repo, nil, nil)
	req.Action = "place_meeple"

	result, err := service.ApplyAction(t.Context(), req)
	if !errors.Is(err, ErrActionIDConflict) {
		t.Fatalf("ApplyAction() error = %v, want ErrActionIDConflict", err)
	}
	if result == nil || !result.Replayed || result.Status != string(model.MatchActionStatusRejected) {
		t.Fatalf("result = %#v, want rejected replay", result)
	}
	if repo.getActiveCalls != 1 {
		t.Fatalf("GetActiveByRoomID calls = %d, want 1", repo.getActiveCalls)
	}
}

func TestMatchActionRequestHashCanonicalizesPayloadAndCoversPreconditions(t *testing.T) {
	base := testReliableActionRequest()
	base.Payload = json.RawMessage(`{"x":1,"nested":{"b":2,"a":1}}`)
	baseHash, err := matchActionRequestHash(base)
	if err != nil {
		t.Fatalf("matchActionRequestHash() error = %v", err)
	}

	reordered := base
	reordered.Payload = json.RawMessage(` { "nested": { "a": 1, "b": 2 }, "x": 1 } `)
	reorderedHash, err := matchActionRequestHash(reordered)
	if err != nil {
		t.Fatalf("matchActionRequestHash(reordered) error = %v", err)
	}
	if reorderedHash != baseHash {
		t.Fatalf("equivalent JSON hashes differ: %s != %s", reorderedHash, baseHash)
	}

	mutations := []struct {
		name   string
		mutate func(*matchDTO.ApplyMatchActionRequest)
	}{
		{"match", func(req *matchDTO.ApplyMatchActionRequest) { req.ExpectedMatchID = "match-2" }},
		{"action", func(req *matchDTO.ApplyMatchActionRequest) { req.Action = "place_meeple" }},
		{"turn", func(req *matchDTO.ApplyMatchActionRequest) { req.ExpectedTurnNumber++ }},
		{"phase", func(req *matchDTO.ApplyMatchActionRequest) { req.ExpectedPhase = "place_meeple" }},
		{"version", func(req *matchDTO.ApplyMatchActionRequest) { req.ExpectedStateVersion++ }},
		{"payload", func(req *matchDTO.ApplyMatchActionRequest) { req.Payload = json.RawMessage(`{"x":2}`) }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			changed := base
			tc.mutate(&changed)
			hash, err := matchActionRequestHash(changed)
			if err != nil {
				t.Fatalf("matchActionRequestHash() error = %v", err)
			}
			if hash == baseHash {
				t.Fatalf("mutation %q did not change request hash", tc.name)
			}
		})
	}
}

func TestPersistActionResultIncrementsVersionAndAttachesTerminalReceipt(t *testing.T) {
	match := testReliableMatch(model.MatchStatusActive, `{"version":9,"phase":"place_meeple","turnNumber":5}`)
	repo := &reliableActionRepo{match: match}
	terminator := &reliableActionTerminator{}
	service := New(repo, terminator, nil)
	receipt := &model.MatchActionReceipt{
		RoomID:      match.RoomID,
		MatchID:     match.ID,
		ActorID:     "actor-1",
		ActionID:    "action-terminal",
		RequestHash: "hash",
		Status:      model.MatchActionStatusAccepted,
	}

	finished, version, err := service.persistActionResult(t.Context(), match.RoomID, "actor-1", match, gameService.ApplyActionResult{
		NextState:  model.JSONB(`{"version":77,"phase":"finished","turnNumber":5}`),
		NextStatus: model.MatchStatusFinished,
		Activities: []model.MatchActivity{
			{ActorID: "actor-1", TurnNumber: 5, Type: "meeple_placed", Payload: model.JSONB(`{}`)},
		},
	}, receipt)
	if err != nil {
		t.Fatalf("persistActionResult() error = %v", err)
	}
	if !finished || version != 10 || receipt.StateVersion != 10 {
		t.Fatalf("finished = %v, version = %d, receipt version = %d; want true, 10, 10", finished, version, receipt.StateVersion)
	}
	if terminator.calls != 1 || terminator.request.ActionReceipt != receipt {
		t.Fatalf("terminator calls = %d, receipt = %#v; want original receipt", terminator.calls, terminator.request.ActionReceipt)
	}
	if terminator.request.ExpectedStateVersion == nil || *terminator.request.ExpectedStateVersion != 9 {
		t.Fatalf("expected terminal state version = %v, want 9", terminator.request.ExpectedStateVersion)
	}
	if got := gameStateVersion(terminator.request.GameState); got != 10 {
		t.Fatalf("terminal game state version = %d, want 10", got)
	}
	if got, want := len(terminator.request.Activities), 1; got != want {
		t.Fatalf("terminal activities = %d, want %d", got, want)
	}
	activity := terminator.request.Activities[0]
	if activity.ID == "" || activity.MatchID != match.ID || activity.StateVersion != 10 || activity.Ordinal != 0 || activity.CreatedAt.IsZero() {
		t.Fatalf("terminal activity = %#v, want stamped match/version/ordinal/id/time", activity)
	}
}

func testReliableActionRequest() matchDTO.ApplyMatchActionRequest {
	return matchDTO.ApplyMatchActionRequest{
		ActionID:             "action-1",
		ActorID:              "actor-1",
		RoomID:               "room-1",
		ExpectedMatchID:      "match-1",
		Action:               "place_tile",
		ExpectedTurnNumber:   1,
		ExpectedPhase:        "place_tile",
		ExpectedStateVersion: 1,
		Payload:              json.RawMessage(`{"x":0,"y":1}`),
	}
}

func testReliableMatch(status model.MatchStatus, state string) *model.Match {
	return &model.Match{
		ID:        "match-1",
		RoomID:    "room-1",
		GameType:  "reliable-action-test",
		Status:    status,
		GameState: model.JSONB(state),
		CreatedAt: time.Unix(1, 0).UTC(),
		UpdatedAt: time.Unix(2, 0).UTC(),
	}
}

type reliableActionRepo struct {
	match                  *model.Match
	activeMatch            *model.Match
	players                []model.MatchPlayer
	receipt                *model.MatchActionReceipt
	getActiveCalls         int
	saveReceiptCalls       int
	updateCalls            int
	updateWithReceiptCalls int
	activities             []model.MatchActivity
	listActivitiesVersion  int
}

func (r *reliableActionRepo) Create(context.Context, *model.Match, []model.MatchPlayer) error {
	return nil
}

func (r *reliableActionRepo) GetByID(_ context.Context, matchID string) (*model.Match, []model.MatchPlayer, error) {
	if r.match == nil || r.match.ID != matchID {
		return nil, nil, matchPostgres.ErrNotFound
	}
	return r.match, r.players, nil
}

func (r *reliableActionRepo) GetActiveByRoomID(_ context.Context, roomID string) (*model.Match, []model.MatchPlayer, error) {
	r.getActiveCalls++
	match := r.activeMatch
	if match == nil {
		match = r.match
	}
	if match == nil || match.RoomID != roomID || match.Status != model.MatchStatusActive {
		return nil, nil, matchPostgres.ErrNotFound
	}
	return match, r.players, nil
}

func (r *reliableActionRepo) GetLastByRoomID(_ context.Context, roomID string) (*model.Match, []model.MatchPlayer, error) {
	if r.match == nil || r.match.RoomID != roomID {
		return nil, nil, matchPostgres.ErrNotFound
	}
	return r.match, r.players, nil
}

func (r *reliableActionRepo) ListRecentActivities(_ context.Context, matchID string, maxStateVersion, limit int) ([]model.MatchActivity, error) {
	r.listActivitiesVersion = maxStateVersion
	result := make([]model.MatchActivity, 0, len(r.activities))
	for _, activity := range r.activities {
		if activity.MatchID == matchID && activity.StateVersion <= maxStateVersion {
			result = append(result, activity)
		}
	}
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (r *reliableActionRepo) GetActionReceipt(_ context.Context, roomID, actorID, actionID string) (*model.MatchActionReceipt, error) {
	if r.receipt == nil || r.receipt.RoomID != roomID || r.receipt.ActorID != actorID || r.receipt.ActionID != actionID {
		return nil, matchPostgres.ErrNotFound
	}
	copy := *r.receipt
	return &copy, nil
}

func (r *reliableActionRepo) SaveActionReceipt(_ context.Context, receipt *model.MatchActionReceipt) error {
	r.saveReceiptCalls++
	if r.receipt != nil {
		return matchPostgres.ErrActionAlreadySaved
	}
	copy := *receipt
	r.receipt = &copy
	return nil
}

func (r *reliableActionRepo) UpdateState(_ context.Context, matchID string, _ int, state model.JSONB, status model.MatchStatus, result *model.JSONB) error {
	r.updateCalls++
	return r.updateMatch(matchID, state, status, result)
}

func (r *reliableActionRepo) UpdateStateWithActionReceipt(_ context.Context, matchID string, _ int, state model.JSONB, status model.MatchStatus, result *model.JSONB, receipt *model.MatchActionReceipt) error {
	r.updateWithReceiptCalls++
	if r.receipt != nil {
		return matchPostgres.ErrActionAlreadySaved
	}
	if err := r.updateMatch(matchID, state, status, result); err != nil {
		return err
	}
	copy := *receipt
	r.receipt = &copy
	return nil
}

func (r *reliableActionRepo) PersistActionResult(_ context.Context, matchID string, _ int, state model.JSONB, status model.MatchStatus, result *model.JSONB, receipt *model.MatchActionReceipt, activities []model.MatchActivity) error {
	if receipt == nil {
		r.updateCalls++
	} else {
		r.updateWithReceiptCalls++
		if r.receipt != nil {
			return matchPostgres.ErrActionAlreadySaved
		}
	}
	if err := r.updateMatch(matchID, state, status, result); err != nil {
		return err
	}
	if receipt != nil {
		copy := *receipt
		r.receipt = &copy
	}
	r.activities = append(r.activities, activities...)
	return nil
}

func (r *reliableActionRepo) updateMatch(matchID string, state model.JSONB, status model.MatchStatus, result *model.JSONB) error {
	if r.match == nil || r.match.ID != matchID {
		return matchPostgres.ErrNotFound
	}
	r.match.GameState = append(model.JSONB(nil), state...)
	r.match.Status = status
	r.match.Result = result
	return nil
}

type reliableActionEngine struct {
	result      gameService.ApplyActionResult
	err         error
	botBuildErr error
	applyCalls  int
}

func (e *reliableActionEngine) GameType() string { return "reliable-action-test" }

func (e *reliableActionEngine) NormalizeRoomSettings(raw json.RawMessage) (model.JSONB, error) {
	return model.JSONB(raw), nil
}

func (e *reliableActionEngine) ValidateRoomConfig(int, model.JSONB) error { return nil }

func (e *reliableActionEngine) BuildInitialMatch(*model.Room, []model.RoomParticipant) (*model.Match, []model.MatchPlayer, error) {
	return nil, nil, nil
}

func (e *reliableActionEngine) ApplyAction(context.Context, *model.Match, []model.MatchPlayer, gameService.ApplyActionRequest) (gameService.ApplyActionResult, error) {
	e.applyCalls++
	return e.result, e.err
}

func (e *reliableActionEngine) ApplyTurnTimeout(context.Context, *model.Match, []model.MatchPlayer) (gameService.ApplyActionResult, error) {
	return e.result, e.err
}

func (e *reliableActionEngine) BuildBotAction(*model.Match, model.MatchPlayer) (gameService.ApplyActionRequest, error) {
	return gameService.ApplyActionRequest{}, e.botBuildErr
}

func (e *reliableActionEngine) BuildPublicState(match *model.Match, _ []model.MatchPlayer) (json.RawMessage, error) {
	return json.RawMessage(match.GameState), nil
}

func (e *reliableActionEngine) BuildPrivateState(match *model.Match, _ []model.MatchPlayer, _ string) (json.RawMessage, error) {
	return json.RawMessage(match.GameState), nil
}

type reliableActionTerminator struct {
	calls   int
	request roomDTO.FinishRoomMatchRequest
}

func (t *reliableActionTerminator) FinishRoomMatch(_ context.Context, req roomDTO.FinishRoomMatchRequest) error {
	t.calls++
	t.request = req
	return nil
}
