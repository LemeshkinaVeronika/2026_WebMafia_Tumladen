package service

import (
	"encoding/json"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

const (
	minPlayersLimit    = 2
	maxPlayersLimit    = 6
	defaultTurnTime    = 120
	minTurnTimeSeconds = 30
	maxTurnTimeSeconds = 300
)

func defaultRoomSettings() carcassonneDTO.RoomSettings {
	return carcassonneDTO.RoomSettings{
		TurnTimeSeconds: defaultTurnTime,
	}
}

func (e *Engine) NormalizeRoomSettings(raw json.RawMessage) (model.JSONB, error) {
	settings := defaultRoomSettings()

	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, gameService.ErrInvalidRoomSettings
		}
	}

	if settings.TurnTimeSeconds != 0 && (settings.TurnTimeSeconds < minTurnTimeSeconds || settings.TurnTimeSeconds > maxTurnTimeSeconds) {
		return nil, gameService.ErrInvalidRoomSettings
	}

	normalized, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	return model.JSONB(normalized), nil
}

func (e *Engine) ValidateRoomConfig(maxPlayers int, _ model.JSONB) error {
	if maxPlayers < minPlayersLimit || maxPlayers > maxPlayersLimit {
		return gameService.ErrInvalidRoomConfig
	}

	return nil
}
