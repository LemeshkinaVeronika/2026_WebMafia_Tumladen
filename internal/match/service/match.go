package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/match/dto"
	matchPostgres "github.com/webmafia/tumladan/internal/match/repository/postgres"
	"github.com/webmafia/tumladan/internal/model"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
)

const (
	maxBotTurnSteps = 32
)

func matchToResponse(match model.Match, players []model.MatchPlayer) dto.MatchResponse {
	resp := dto.MatchResponse{
		ID:        match.ID,
		RoomID:    match.RoomID,
		GameType:  match.GameType,
		Status:    string(match.Status),
		GameState: json.RawMessage(match.GameState),
		Players:   make([]dto.MatchPlayerResponse, 0, len(players)),
		CreatedAt: match.CreatedAt.Format(time.RFC3339),
		UpdatedAt: match.UpdatedAt.Format(time.RFC3339),
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
		terminatedAt := match.TerminatedAt.Format(time.RFC3339)
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

func (s *Service) GetActiveByRoomID(ctx context.Context, roomID string) (*dto.MatchResponse, error) {
	match, players, err := s.repo.GetActiveByRoomID(ctx, roomID)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}

	resp := matchToResponse(*match, players)
	return &resp, nil
}

func (s *Service) ApplyAction(ctx context.Context, req dto.ApplyMatchActionRequest) (*dto.MatchResponse, error) {
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

	actionResult, err := s.games.ApplyAction(ctx, match, players, gameService.ApplyActionRequest{
		ActorID: req.ActorID,
		Action:  req.Action,
		Payload: req.Payload,
	})
	if err != nil {
		return nil, mapGameMatchError(err)
	}

	finished, err := s.persistActionResult(ctx, req.RoomID, req.ActorID, match, actionResult)
	if err != nil {
		return nil, err
	}
	if finished {
		return s.GetLastByRoomID(ctx, req.RoomID)
	}

	return s.applyAutomaticBotTurns(ctx, req.RoomID, match, players)
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
		return nil, ErrInvalidMatchAction
	}

	actionResult, err := s.games.ApplyTurnTimeout(ctx, match, players)
	if err != nil {
		return nil, mapGameMatchError(err)
	}

	finished, err := s.persistActionResult(ctx, req.RoomID, req.ExpectedActorID, match, actionResult)
	if err != nil {
		return nil, err
	}
	if finished {
		return s.GetLastByRoomID(ctx, req.RoomID)
	}

	return s.applyAutomaticBotTurns(ctx, req.RoomID, match, players)
}

func (s *Service) persistActionResult(
	ctx context.Context,
	roomID string,
	actorID string,
	match *model.Match,
	actionResult gameService.ApplyActionResult,
) (bool, error) {
	if actionResult.NextStatus == model.MatchStatusFinished {
		if s.terminator == nil {
			return false, ErrInvalidMatchAction
		}

		if err := s.terminator.FinishRoomMatch(ctx, roomDTO.FinishRoomMatchRequest{
			ActorID:   &actorID,
			RoomID:    roomID,
			Reason:    string(model.MatchTerminationReasonNormalCompletion),
			GameState: json.RawMessage(actionResult.NextState),
			Result:    json.RawMessage(resultOrNull(actionResult.Result)),
		}); err != nil {
			return false, err
		}

		match.GameState = actionResult.NextState
		match.Status = actionResult.NextStatus
		match.Result = actionResult.Result
		return true, nil
	}

	if err := s.repo.UpdateState(ctx, match.ID, actionResult.NextState, actionResult.NextStatus, actionResult.Result); err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return false, ErrMatchNotFound
		}
		return false, err
	}

	match.GameState = actionResult.NextState
	match.Status = actionResult.NextStatus
	match.Result = actionResult.Result
	return false, nil
}

func (s *Service) applyAutomaticBotTurns(
	ctx context.Context,
	roomID string,
	match *model.Match,
	players []model.MatchPlayer,
) (*dto.MatchResponse, error) {
	for step := 0; step < maxBotTurnSteps; step++ {
		actorID, ok := currentTurnActorID(match)
		if !ok {
			return s.GetActiveByRoomID(ctx, roomID)
		}

		player, ok := currentTurnBotPlayer(players, actorID)
		if !ok {
			return s.GetActiveByRoomID(ctx, roomID)
		}

		req, err := s.games.BuildBotAction(match, player)
		if err != nil {
			return nil, mapGameMatchError(err)
		}

		actionResult, err := s.games.ApplyAction(ctx, match, players, req)
		if err != nil {
			return nil, mapGameMatchError(err)
		}

		finished, err := s.persistActionResult(ctx, roomID, actorID, match, actionResult)
		if err != nil {
			return nil, err
		}
		if finished {
			return s.GetLastByRoomID(ctx, roomID)
		}
	}

	return nil, ErrInvalidMatchAction
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

func (s *Service) GetLastByRoomID(ctx context.Context, roomID string) (*dto.MatchResponse, error) {
	match, players, err := s.repo.GetLastByRoomID(ctx, roomID)
	if err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}

	resp := matchToResponse(*match, players)
	return &resp, nil
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
