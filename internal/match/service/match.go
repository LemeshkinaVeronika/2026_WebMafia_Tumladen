package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/webmafia/tumladan/internal/match/dto"
	matchPostgres "github.com/webmafia/tumladan/internal/match/repository/postgres"
	"github.com/webmafia/tumladan/internal/model"
	roomDTO "github.com/webmafia/tumladan/internal/room/dto"
	"time"
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
			DisplayName:    player.DisplayName,
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

func BuildInitialMatch(room *model.Room, participants []model.RoomParticipant) (*model.Match, []model.MatchPlayer, error) {
	switch room.GameType {
	case "carcassonne":
		return buildInitialCarcassonneMatch(room, participants)
	default:
		return nil, nil, fmt.Errorf("unsupported game type: %s", room.GameType)
	}
}

//TODO:убрать заглушку, вынести в отдельный слой

func buildInitialCarcassonneMatch(room *model.Room, participants []model.RoomParticipant) (*model.Match, []model.MatchPlayer, error) {
	now := time.Now().UTC()
	matchID := uuid.NewString()

	settings := CarcassonneMatchSettings{
		TurnTimeSeconds: 60,
	}

	players := make([]model.MatchPlayer, 0, len(participants))
	statePlayers := make([]CarcassonnePlayerState, 0, len(participants))

	for i, participant := range participants {
		players = append(players, model.MatchPlayer{
			MatchID:     matchID,
			ActorID:     participant.ActorID,
			DisplayName: participant.DisplayName,
			Seat:        i,
		})

		statePlayers = append(statePlayers, CarcassonnePlayerState{
			ActorID:     participant.ActorID,
			DisplayName: participant.DisplayName,
			Seat:        i,
			Score:       0,
			MeeplesLeft: 7,
		})
	}

	currentPlayerID := ""
	if len(statePlayers) > 0 {
		currentPlayerID = statePlayers[0].ActorID
	}

	gameState := CarcassonneGameState{
		Version:         1,
		Phase:           "tile_placement",
		TurnNumber:      1,
		CurrentPlayerID: currentPlayerID,
		Players:         statePlayers,
		Board:           []any{},
		DeckRemaining:   0,
		Settings:        settings,
	}

	rawState, err := json.Marshal(gameState)
	if err != nil {
		return nil, nil, err
	}

	match := &model.Match{
		ID:        matchID,
		RoomID:    room.ID,
		GameType:  room.GameType,
		Status:    model.MatchStatusActive,
		GameState: model.JSONB(rawState),
		Result:    nil,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return match, players, nil
}

func (s *Service) ApplyAction(ctx context.Context, req dto.ApplyMatchActionRequest) (*dto.MatchResponse, error) {
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

	var (
		nextState  model.JSONB
		nextStatus = match.Status
		result     *model.JSONB
	)

	switch match.GameType {
	case "carcassonne":
		nextState, nextStatus, result, err = applyCarcassonneAction(match.GameState, req)
		if err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidMatchAction
	}

	if nextStatus == model.MatchStatusFinished {
		if s.terminator == nil {
			return nil, ErrInvalidMatchAction
		}

		if err := s.terminator.FinishRoomMatch(ctx, roomDTO.FinishRoomMatchRequest{
			ActorID: &req.ActorID,
			RoomID:  req.RoomID,
			Reason:  string(model.MatchTerminationReasonNormalCompletion),
			Result:  json.RawMessage(resultOrNull(result)),
		}); err != nil {
			return nil, err
		}

		return s.GetLastByRoomID(ctx, req.RoomID)
	}

	if err := s.repo.UpdateState(ctx, match.ID, nextState, nextStatus, result); err != nil {
		if errors.Is(err, matchPostgres.ErrNotFound) {
			return nil, ErrMatchNotFound
		}
		return nil, err
	}

	match.GameState = nextState
	match.Status = nextStatus
	match.Result = result

	resp := matchToResponse(*match, players)
	return &resp, nil
}

func resultOrNull(result *model.JSONB) []byte {
	if result == nil {
		return []byte(`null`)
	}

	return *result
}

func applyCarcassonneAction(rawState model.JSONB, req dto.ApplyMatchActionRequest) (model.JSONB, model.MatchStatus, *model.JSONB, error) {
	var state CarcassonneGameState
	if err := json.Unmarshal(rawState, &state); err != nil {
		return nil, model.MatchStatusActive, nil, ErrInvalidMatchAction
	}

	switch req.Action {
	case "advance_turn":
		return applyCarcassonneAdvanceTurn(state, req.ActorID)
	default:
		return nil, model.MatchStatusActive, nil, ErrInvalidMatchAction
	}
}

func applyCarcassonneAdvanceTurn(state CarcassonneGameState, actorID string) (model.JSONB, model.MatchStatus, *model.JSONB, error) {
	if len(state.Players) == 0 {
		return nil, model.MatchStatusActive, nil, ErrInvalidMatchAction
	}

	if state.CurrentPlayerID != actorID {
		return nil, model.MatchStatusActive, nil, ErrNotYourTurn
	}

	currentIndex := -1
	for i, player := range state.Players {
		if player.ActorID == state.CurrentPlayerID {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 {
		return nil, model.MatchStatusActive, nil, ErrInvalidMatchAction
	}

	nextIndex := (currentIndex + 1) % len(state.Players)
	state.CurrentPlayerID = state.Players[nextIndex].ActorID
	state.TurnNumber++

	normalized, err := json.Marshal(state)
	if err != nil {
		return nil, model.MatchStatusActive, nil, err
	}

	return model.JSONB(normalized), model.MatchStatusActive, nil, nil
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
