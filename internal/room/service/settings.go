package service

import (
	"encoding/json"

	"github.com/webmafia/tumladan/internal/model"
	"github.com/webmafia/tumladan/internal/room/dto"
)

func defaultCarcassonneSettings() dto.CarcassonneRoomSettings {
	return dto.CarcassonneRoomSettings{
		TurnTimeSeconds: carcassonneDefaultTurnTime,
	}
}

func normalizeRoomSettings(gameType string, raw json.RawMessage) (model.JSONB, error) {
	switch gameType {
	case "carcassonne":
		return normalizeCarcassonneSettings(raw)
	default:
		return nil, ErrInvalidGameType
	}
}

func normalizeCarcassonneSettings(raw json.RawMessage) (model.JSONB, error) {
	settings := defaultCarcassonneSettings()

	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, ErrInvalidRoomSettings
		}
	}

	if settings.TurnTimeSeconds < carcassonneMinTurnTime || settings.TurnTimeSeconds > carcassonneMaxTurnTime {
		return nil, ErrInvalidRoomSettings
	}

	normalized, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	return model.JSONB(normalized), nil
}
