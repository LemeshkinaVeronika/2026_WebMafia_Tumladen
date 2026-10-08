package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/google/uuid"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/match/dto"
	matchPostgres "github.com/webmafia/tumladan/internal/match/repository/postgres"
	"github.com/webmafia/tumladan/internal/model"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
	roomService "github.com/webmafia/tumladan/internal/room/service"
)

const (
	maxBotTurnSteps          = 32
	maxRecentMatchActivities = 50
)

func matchToResponse(match model.Match, players []model.MatchPlayer) dto.MatchResponse {
	resp := dto.MatchResponse{
		ID:        match.ID,
		RoomID:    match.RoomID,
		GameType:  match.GameType,
		Status:    string(match.Status),
		GameState: json.RawMessage(match.GameState),
		Players:   make([]dto.MatchPlayerResponse, 0, len(players)),
		CreatedAt: match.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt: match.UpdatedAt.Format(time.RFC3339Nano),
	}

	if match.Result != nil {
		resp.Result = json.RawMessage(*match.Result)
	}
	if match.TerminationReason != nil {
		reason := string(*match.TerminationReason)
		resp.TerminationReason = &reason
	}
	if match.TerminatedByActorID != nil {
		resp.TerminatedByActorID = match.TerminatedByActorID
	}
	if match.TerminatedAt != nil {
		terminatedAt := match.TerminatedAt.Format(time.RFC3339Nano)
		resp.TerminatedAt = &terminatedAt
	}

	for _, player := range players {
		resp.Players = append(resp.Players, dto.MatchPlayerResponse{
			ActorID:        player.ActorID,
			ActorType:      string(player.ActorType),
			DisplayName:    player.DisplayName,
			BotDifficulty:  player.BotDifficulty,
			AvatarURL:      player.AvatarURL,
			Seat:           player.Seat,
			IsDisconnected: player.DisconnectedAt != nil,
		})
	}

	return resp
}

func (s *Service) matchResponse(ctx context.Context, match model.Match, players []model.MatchPlayer) (*dto.MatchResponse, error) {
	activities, err := s.repo.ListRecentActivities(ctx, match.ID, gameStateVersion(match.GameState), maxRecentMatchActivities)
	if err != nil {
		return nil, err
	}

	response := matchToResponse(match, players)
	response.RecentActions = make([]dto.MatchActivityResponse, 0, len(activities))
	for _, activity := range activities {
		response.RecentActions = append(response.RecentActions, dto.MatchActivityResponse{
			ID:           activity.ID,
			StateVersion: activity.StateVersion,
			TurnNumber:   activity.TurnNumber,
			ActorID:      activity.ActorID,
			Type:         activity.Type,
			Payload:      json.RawMessage(activity.Payload),
			CreatedAt:    activity.CreatedAt.Format(time.RFC3339Nano),
		})
	}

	return &response, nil
}

func (s *Service) GetActiveByRoomID(ctx context.Context, roomID string) (*dto.MatchResponse, error) {
	match, players, err := s.repo.GetActiveByRoomID(ctx, roomID)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}

	return s.matchResponse(ctx, *match, players)
}

func (s *Service) ApplyAction(ctx context.Context, req dto.ApplyMatchActionRequest) (*dto.ApplyMatchActionResult, error) {
	lockedCtx, unlock, err := s.roomLocks.Lock(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = lockedCtx

	requestHash, err := matchActionRequestHash(req)
	if err != nil {
		return nil, ErrInvalidMatchAction
	}

	receipt, err := s.repo.GetActionReceipt(ctx, req.RoomID, req.ActorID, req.ActionID)
	if err == nil {
		return s.replayMatchAction(ctx, receipt, requestHash)
	}
	if !errors.Is(err, matchPostgres.ErrNotFound) {
		return nil, err
	}

	match, players, err := s.repo.GetActiveByRoomID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}

	if match.Status != model.MatchStatusActive {
		return nil, ErrMatchNotActive
	}
	if !matchesExpectedActionState(match, req) {
		return s.rejectMatchAction(ctx, match, players, req, requestHash, ErrMatchStateConflict)
	}

	actionResult, err := s.games.ApplyAction(ctx, match, players, gameService.ApplyActionRequest{
		ActorID: req.ActorID,
		Action:  req.Action,
		Payload: req.Payload,
	})
	if err != nil {
		mappedErr := mapGameMatchError(err)
		if isDurableMatchActionRejection(mappedErr) {
			return s.rejectMatchAction(ctx, match, players, req, requestHash, mappedErr)
		}
		return nil, mappedErr
	}

	actionReceipt := &model.MatchActionReceipt{
		RoomID:      req.RoomID,
		MatchID:     match.ID,
		ActorID:     req.ActorID,
		ActionID:    req.ActionID,
		RequestHash: requestHash,
		Status:      model.MatchActionStatusAccepted,
	}
	finished, actionStateVersion, err := s.persistActionResult(ctx, req.RoomID, req.ActorID, match, actionResult, actionReceipt)
	if err != nil {
		stored, receiptErr := s.repo.GetActionReceipt(ctx, req.RoomID, req.ActorID, req.ActionID)
		if receiptErr == nil {
			return s.replayMatchAction(ctx, stored, requestHash)
		}
		if errors.Is(err, ErrMatchStateConflict) {
			currentMatch, currentPlayers, currentErr := s.repo.GetByID(ctx, match.ID)
			if currentErr == nil {
				return s.rejectMatchAction(ctx, currentMatch, currentPlayers, req, requestHash, ErrMatchStateConflict)
			}
		}
		return nil, err
	}
	events := append([]gameService.GameEvent(nil), actionResult.Events...)
	if finished {
		response, err := s.matchStateByID(ctx, match.ID)
		return &dto.ApplyMatchActionResult{
			MatchState:   attachMatchEvents(response, events),
			Status:       string(model.MatchActionStatusAccepted),
			StateVersion: actionStateVersion,
		}, err
	}

	response, err := s.applyAutomaticBotTurns(ctx, req.RoomID, match, players, events)
	if err != nil && response == nil {
		// The human action and its receipt are already committed at this point.
		// Always return the latest successfully persisted in-memory snapshot so
		// the transport can broadcast it and restore the turn timer even when a
		// following automatic bot step fails.
		response, _ = s.matchResponse(ctx, *match, players)
		if response == nil {
			response = ptrMatchResponse(matchToResponse(*match, players))
		}
		response = attachMatchEvents(response, events)
	}
	return &dto.ApplyMatchActionResult{
		MatchState:   response,
		Status:       string(model.MatchActionStatusAccepted),
		StateVersion: actionStateVersion,
	}, err
}

func (s *Service) ApplyTurnTimeout(ctx context.Context, req dto.ApplyTurnTimeoutRequest) (*dto.MatchResponse, error) {
	lockedCtx, unlock, err := s.roomLocks.Lock(ctx, req.RoomID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = lockedCtx

	match, players, err := s.repo.GetActiveByRoomID(ctx, req.RoomID)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}

	if match.Status != model.MatchStatusActive {
		return nil, ErrMatchNotActive
	}
	if !matchesExpectedTimeoutState(match, req) {
		return nil, ErrMatchStateConflict
	}

	actionResult, err := s.games.ApplyTurnTimeout(ctx, match, players)
	if err != nil {
		return nil, mapGameMatchError(err)
	}

	finished, _, err := s.persistActionResult(ctx, req.RoomID, req.ExpectedActorID, match, actionResult, nil)
	if err != nil {
		return nil, err
	}
	events := append([]gameService.GameEvent(nil), actionResult.Events...)
	if finished {
		response, err := s.matchStateByID(ctx, match.ID)
		return attachMatchEvents(response, events), err
	}

	return s.applyAutomaticBotTurns(ctx, req.RoomID, match, players, events)
}

func (s *Service) persistActionResult(
	ctx context.Context,
	roomID string,
	actorID string,
	match *model.Match,
	actionResult gameService.ApplyActionResult,
	actionReceipt *model.MatchActionReceipt,
) (bool, int, error) {
	nextState, stateVersion, err := advanceGameStateVersion(match.GameState, actionResult.NextState)
	if err != nil {
		return false, 0, err
	}
	actionResult.NextState = nextState
	if actionReceipt != nil {
		actionReceipt.StateVersion = stateVersion
	}
	activities := prepareMatchActivities(match.ID, stateVersion, actionResult.Activities)

	if actionResult.NextStatus == model.MatchStatusFinished {
		if s.terminator == nil {
			return false, 0, ErrInvalidMatchAction
		}

		expectedStateVersion := stateVersion - 1
		if err := s.terminator.FinishRoomMatch(ctx, roomDTO.FinishRoomMatchRequest{
			ActorID:              &actorID,
			RoomID:               roomID,
			Reason:               string(model.MatchTerminationReasonNormalCompletion),
			GameState:            json.RawMessage(actionResult.NextState),
			Result:               json.RawMessage(resultOrNull(actionResult.Result)),
			ExpectedStateVersion: &expectedStateVersion,
			ActionReceipt:        actionReceipt,
			Activities:           activities,
		}); err != nil {
			if errors.Is(err, roomService.ErrMatchStateConflict) {
				return false, 0, ErrMatchStateConflict
			}
			return false, 0, err
		}

		match.GameState = actionResult.NextState
		match.Status = actionResult.NextStatus
		match.Result = actionResult.Result
		return true, stateVersion, nil
	}

	err = s.repo.PersistActionResult(
		ctx,
		match.ID,
		stateVersion-1,
		actionResult.NextState,
		actionResult.NextStatus,
		actionResult.Result,
		actionReceipt,
		activities,
	)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrStateConflict) {
			return false, 0, ErrMatchStateConflict
		}
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return false, 0, ErrMatchNotFound
		}
		return false, 0, err
	}

	match.GameState = actionResult.NextState
	match.Status = actionResult.NextStatus
	match.Result = actionResult.Result
	return false, stateVersion, nil
}

func prepareMatchActivities(matchID string, stateVersion int, activities []model.MatchActivity) []model.MatchActivity {
	if len(activities) == 0 {
		return nil
	}

	prepared := make([]model.MatchActivity, len(activities))
	now := time.Now().UTC()
	for i, activity := range activities {
		activity.MatchID = matchID
		activity.StateVersion = stateVersion
		activity.Ordinal = i
		if activity.ID == "" {
			activity.ID = uuid.NewString()
		}
		if activity.CreatedAt.IsZero() {
			activity.CreatedAt = now
		}
		if len(activity.Payload) == 0 {
			activity.Payload = model.JSONB(`{}`)
		}
		prepared[i] = activity
	}
	return prepared
}

func (s *Service) applyAutomaticBotTurns(
	ctx context.Context,
	roomID string,
	match *model.Match,
	players []model.MatchPlayer,
	events []gameService.GameEvent,
) (*dto.MatchResponse, error) {
	for step := 0; step < maxBotTurnSteps; step++ {
		actorID, ok := currentTurnActorID(match)
		if !ok {
			response, err := s.GetActiveByRoomID(ctx, roomID)
			return attachMatchEvents(response, events), err
		}

		player, ok := currentTurnBotPlayer(players, actorID)
		if !ok {
			response, err := s.GetActiveByRoomID(ctx, roomID)
			return attachMatchEvents(response, events), err
		}

		req, err := s.games.BuildBotAction(match, player)
		if err != nil {
			return nil, mapGameMatchError(err)
		}

		actionResult, err := s.games.ApplyAction(ctx, match, players, req)
		if err != nil {
			return nil, mapGameMatchError(err)
		}

		finished, _, err := s.persistActionResult(ctx, roomID, actorID, match, actionResult, nil)
		if err != nil {
			return nil, err
		}
		events = append(events, actionResult.Events...)
		if finished {
			response, err := s.matchStateByID(ctx, match.ID)
			return attachMatchEvents(response, events), err
		}
	}

	return nil, ErrInvalidMatchAction
}

func attachMatchEvents(response *dto.MatchResponse, events []gameService.GameEvent) *dto.MatchResponse {
	if response == nil || len(events) == 0 {
		return response
	}
	response.Events = append([]gameService.GameEvent(nil), events...)
	return response
}

func currentTurnActorID(match *model.Match) (string, bool) {
	var state struct {
		Phase           string `json:"phase"`
		CurrentPlayerID string `json:"currentPlayerId"`
	}
	if match == nil || json.Unmarshal(match.GameState, &state) != nil {
		return "", false
	}
	if state.CurrentPlayerID == "" {
		return "", false
	}
	if state.Phase != "place_tile" && state.Phase != "place_meeple" {
		return "", false
	}
	return state.CurrentPlayerID, true
}

func currentTurnBotPlayer(players []model.MatchPlayer, actorID string) (model.MatchPlayer, bool) {
	for _, player := range players {
		if player.ActorID == actorID && player.ActorType == model.ActorTypeBot {
			return player, true
		}
	}
	return model.MatchPlayer{}, false
}

func matchesExpectedTimeoutState(match *model.Match, req dto.ApplyTurnTimeoutRequest) bool {
	if match.ID != req.ExpectedMatchID {
		return false
	}

	var state struct {
		Version         int    `json:"version"`
		Phase           string `json:"phase"`
		TurnNumber      int    `json:"turnNumber"`
		CurrentPlayerID string `json:"currentPlayerId"`
	}
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return false
	}

	return state.Version == req.ExpectedStateVersion &&
		state.Phase == req.ExpectedPhase &&
		state.TurnNumber == req.ExpectedTurnNumber &&
		state.CurrentPlayerID == req.ExpectedActorID
}

func resultOrNull(result *model.JSONB) []byte {
	if result == nil {
		return []byte(`null`)
	}

	return *result
}

func (s *Service) replayMatchAction(
	ctx context.Context,
	receipt *model.MatchActionReceipt,
	requestHash string,
) (*dto.ApplyMatchActionResult, error) {
	matchState, stateErr := s.matchStateForReceiptReplay(ctx, receipt)

	if receipt.RequestHash != requestHash {
		stateVersion := receipt.StateVersion
		if matchState != nil {
			stateVersion = gameStateVersion(matchState.GameState)
		}
		return &dto.ApplyMatchActionResult{
			MatchState:   matchState,
			Status:       string(model.MatchActionStatusRejected),
			StateVersion: stateVersion,
			Replayed:     true,
		}, ErrActionIDConflict
	}

	result := &dto.ApplyMatchActionResult{
		MatchState:   matchState,
		Status:       string(receipt.Status),
		StateVersion: receipt.StateVersion,
		ErrorCode:    receipt.ErrorCode,
		ErrorMessage: receipt.ErrorMessage,
		Replayed:     true,
	}
	if receipt.Status == model.MatchActionStatusAccepted {
		return result, stateErr
	}
	return result, matchActionErrorFromCode(receipt.ErrorCode)
}

func (s *Service) matchStateForReceiptReplay(
	ctx context.Context,
	receipt *model.MatchActionReceipt,
) (*dto.MatchResponse, error) {
	match, players, err := s.repo.GetActiveByRoomID(ctx, receipt.RoomID)
	if err == nil {
		for _, player := range players {
			if player.ActorID == receipt.ActorID {
				return s.matchResponse(ctx, *match, players)
			}
		}
		return s.matchStateByID(ctx, receipt.MatchID)
	}
	if !errors.Is(err, matchPostgres.ErrNotFound) {
		return nil, err
	}

	return s.matchStateByID(ctx, receipt.MatchID)
}

func (s *Service) rejectMatchAction(
	ctx context.Context,
	match *model.Match,
	players []model.MatchPlayer,
	req dto.ApplyMatchActionRequest,
	requestHash string,
	rejection error,
) (*dto.ApplyMatchActionResult, error) {
	code, message := matchActionErrorDetails(rejection)
	receipt := &model.MatchActionReceipt{
		RoomID:       req.RoomID,
		MatchID:      match.ID,
		ActorID:      req.ActorID,
		ActionID:     req.ActionID,
		RequestHash:  requestHash,
		Status:       model.MatchActionStatusRejected,
		StateVersion: gameStateVersion(match.GameState),
		ErrorCode:    code,
		ErrorMessage: message,
	}
	if err := s.repo.SaveActionReceipt(ctx, receipt); err != nil {
		if errors.Is(err, matchPostgres.ErrActionAlreadySaved) {
			stored, getErr := s.repo.GetActionReceipt(ctx, req.RoomID, req.ActorID, req.ActionID)
			if getErr != nil {
				return nil, getErr
			}
			return s.replayMatchAction(ctx, stored, requestHash)
		}
		return nil, err
	}

	response, err := s.matchResponse(ctx, *match, players)
	if err != nil {
		return nil, err
	}
	return &dto.ApplyMatchActionResult{
		MatchState:   response,
		Status:       string(model.MatchActionStatusRejected),
		StateVersion: receipt.StateVersion,
		ErrorCode:    receipt.ErrorCode,
		ErrorMessage: receipt.ErrorMessage,
	}, rejection
}

func (s *Service) matchStateByID(ctx context.Context, matchID string) (*dto.MatchResponse, error) {
	match, players, err := s.repo.GetByID(ctx, matchID)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}
	return s.matchResponse(ctx, *match, players)
}

func ptrMatchResponse(response dto.MatchResponse) *dto.MatchResponse {
	return &response
}

func matchesExpectedActionState(match *model.Match, req dto.ApplyMatchActionRequest) bool {
	if match == nil || match.ID != req.ExpectedMatchID {
		return false
	}

	var state struct {
		Version    int    `json:"version"`
		Phase      string `json:"phase"`
		TurnNumber int    `json:"turnNumber"`
	}
	if json.Unmarshal(match.GameState, &state) != nil {
		return false
	}

	return state.Version == req.ExpectedStateVersion &&
		state.Phase == req.ExpectedPhase &&
		state.TurnNumber == req.ExpectedTurnNumber
}

func matchActionRequestHash(req dto.ApplyMatchActionRequest) (string, error) {
	if req.ActionID == "" || len(req.ActionID) > 128 || req.ActorID == "" || req.RoomID == "" ||
		req.ExpectedMatchID == "" || req.Action == "" || req.ExpectedPhase == "" {
		return "", ErrInvalidMatchAction
	}

	canonicalPayload, err := canonicalJSON(req.Payload)
	if err != nil {
		return "", err
	}
	request := struct {
		RoomID               string          `json:"roomId"`
		ExpectedMatchID      string          `json:"expectedMatchId"`
		Action               string          `json:"action"`
		ExpectedTurnNumber   int             `json:"expectedTurnNumber"`
		ExpectedPhase        string          `json:"expectedPhase"`
		ExpectedStateVersion int             `json:"expectedStateVersion"`
		Payload              json.RawMessage `json:"payload"`
	}{
		RoomID:               req.RoomID,
		ExpectedMatchID:      req.ExpectedMatchID,
		Action:               req.Action,
		ExpectedTurnNumber:   req.ExpectedTurnNumber,
		ExpectedPhase:        req.ExpectedPhase,
		ExpectedStateVersion: req.ExpectedStateVersion,
		Payload:              canonicalPayload,
	}
	data, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}

func canonicalJSON(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`null`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, ErrInvalidMatchAction
	}
	return json.Marshal(value)
}

func advanceGameStateVersion(currentRaw, nextRaw model.JSONB) (model.JSONB, int, error) {
	var currentState map[string]json.RawMessage
	if err := json.Unmarshal(currentRaw, &currentState); err != nil {
		return nil, 0, ErrInvalidMatchAction
	}
	var current int
	if version, ok := currentState["version"]; !ok || json.Unmarshal(version, &current) != nil || current < 0 || current == math.MaxInt {
		return nil, 0, ErrInvalidMatchAction
	}

	var nextState map[string]json.RawMessage
	if err := json.Unmarshal(nextRaw, &nextState); err != nil {
		return nil, 0, ErrInvalidMatchAction
	}
	var ignoredVersion int
	if version, ok := nextState["version"]; !ok || json.Unmarshal(version, &ignoredVersion) != nil {
		return nil, 0, ErrInvalidMatchAction
	}
	next := current + 1
	encodedVersion, err := json.Marshal(next)
	if err != nil {
		return nil, 0, err
	}
	nextState["version"] = encodedVersion
	encodedState, err := json.Marshal(nextState)
	if err != nil {
		return nil, 0, err
	}
	return model.JSONB(encodedState), next, nil
}

func gameStateVersion(raw []byte) int {
	var state struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(raw, &state) != nil {
		return 0
	}
	return state.Version
}

func isDurableMatchActionRejection(err error) bool {
	return errors.Is(err, ErrInvalidMatchAction) ||
		errors.Is(err, ErrNotYourTurn) ||
		errors.Is(err, ErrMatchStateConflict)
}

func matchActionErrorDetails(err error) (string, string) {
	switch {
	case errors.Is(err, ErrNotYourTurn):
		return "NOT_YOUR_TURN", "not your turn"
	case errors.Is(err, ErrMatchStateConflict):
		return "MATCH_STATE_CONFLICT", "match state changed"
	case errors.Is(err, ErrActionIDConflict):
		return "ACTION_ID_CONFLICT", "action id was already used"
	default:
		return "INVALID_MATCH_ACTION", "invalid match action"
	}
}

func matchActionErrorFromCode(code string) error {
	switch code {
	case "NOT_YOUR_TURN":
		return ErrNotYourTurn
	case "MATCH_STATE_CONFLICT":
		return ErrMatchStateConflict
	case "ACTION_ID_CONFLICT":
		return ErrActionIDConflict
	default:
		return ErrInvalidMatchAction
	}
}

func (s *Service) GetLastByRoomID(ctx context.Context, roomID string) (*dto.MatchResponse, error) {
	match, players, err := s.repo.GetLastByRoomID(ctx, roomID)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}

	return s.matchResponse(ctx, *match, players)
}

func mapGameMatchError(err error) error {
	switch {
	case errors.Is(err, gameService.ErrInvalidMatchAction), errors.Is(err, gameService.ErrUnsupportedGameType):
		return ErrInvalidMatchAction
	case errors.Is(err, gameService.ErrNotYourTurn):
		return ErrNotYourTurn
	default:
		return err
	}
}
