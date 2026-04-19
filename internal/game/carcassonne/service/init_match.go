package service

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	"github.com/webmafia/tumladan/internal/model"
)

const defaultMeeplesPerGuy = 7

func (e *Engine) BuildInitialMatch(room *model.Room, participants []model.RoomParticipant) (*model.Match, []model.MatchPlayer, error) {
	now := time.Now().UTC()
	matchID := uuid.NewString()

	var roomSettings carcassonneDTO.RoomSettings
	if err := json.Unmarshal(room.Settings, &roomSettings); err != nil {
		return nil, nil, err
	}

	players := make([]model.MatchPlayer, 0, len(participants))
	statePlayers := make([]carcassonneDTO.PlayerState, 0, len(participants))

	for i, participant := range participants {
		players = append(players, model.MatchPlayer{
			MatchID:     matchID,
			ActorID:     participant.ActorID,
			DisplayName: participant.DisplayName,
			Seat:        i,
		})

		statePlayers = append(statePlayers, carcassonneDTO.PlayerState{
			ActorID:     participant.ActorID,
			DisplayName: participant.DisplayName,
			Seat:        i,
			Score:       0,
			MeeplesLeft: defaultMeeplesPerGuy,
		})
	}

	currentPlayerID := ""
	if len(statePlayers) > 0 {
		currentPlayerID = statePlayers[0].ActorID
	}

	gameState := carcassonneDTO.GameState{
		Version:         1,
		Phase:           "tile_placement",
		TurnNumber:      1,
		CurrentPlayerID: currentPlayerID,
		Players:         statePlayers,
		Board:           []carcassonneDTO.PlacedTile{},
		CurrentTile:     nil,
		DeckRemaining:   0,
		Settings: carcassonneDTO.MatchSettings{
			TurnTimeSeconds: roomSettings.TurnTimeSeconds,
		},
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
		CreatedAt: now,
		UpdatedAt: now,
	}

	return match, players, nil
}
