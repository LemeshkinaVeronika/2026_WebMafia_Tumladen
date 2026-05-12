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
			ActorType:   participant.ActorType,
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

	startTile := carcassonneDTO.PlacedTile{
		InstanceID: uuid.NewString(),
		TileID:     e.catalog.StartTileID(),
		X:          0,
		Y:          0,
		Rotation:   0,
		PlacedBy:   "",
		TurnNumber: 0,
	}

	deck := buildDeck(e.catalog.BaseDeck())
	shuffleDeck(deck)

	currentTile, deckRemaining := e.drawNextPlaceableTile(deck, []carcassonneDTO.PlacedTile{startTile})
	phase := carcassonneDTO.PhasePlaceTile
	status := model.MatchStatusActive
	var result *model.JSONB
	if currentTile == nil {
		phase = carcassonneDTO.PhaseFinished
		currentPlayerID = ""
		status = model.MatchStatusFinished

		rawResult, err := json.Marshal(matchResult(statePlayers))
		if err != nil {
			return nil, nil, err
		}
		jsonResult := model.JSONB(rawResult)
		result = &jsonResult
	}

	gameState := carcassonneDTO.GameState{
		Version:         1,
		Phase:           phase,
		TurnNumber:      1,
		TurnStartedAt:   now.Format(time.RFC3339),
		CurrentPlayerID: currentPlayerID,
		Players:         statePlayers,
		Board:           []carcassonneDTO.PlacedTile{startTile},
		Settings: carcassonneDTO.MatchSettings{
			TurnTimeSeconds: roomSettings.TurnTimeSeconds,
		},
		DeckRemaining:  deckRemaining,
		CurrentTile:    currentTile,
		LastPlacedTile: nil,
		Meeples:        []carcassonneDTO.PlacedMeeple{},
	}

	rawState, err := json.Marshal(gameState)
	if err != nil {
		return nil, nil, err
	}

	match := &model.Match{
		ID:        matchID,
		RoomID:    room.ID,
		GameType:  room.GameType,
		Status:    status,
		GameState: model.JSONB(rawState),
		Result:    result,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return match, players, nil
}
