package service

import (
	"encoding/json"
	"strings"
	"time"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

const (
	minPlayersLimit            = 2
	maxPlayersLimit            = 6
	defaultTurnTime            = 120
	minTurnTimeSeconds         = 30
	maxTurnTimeSeconds         = 300
	ExpansionInnsAndCathedrals = "inns_and_cathedrals"
)

func defaultRoomSettings() carcassonneDTO.RoomSettings {
	return carcassonneDTO.RoomSettings{
		TurnTimeSeconds: defaultTurnTime,
		Bots:            []carcassonneDTO.BotSettings{},
		Expansions:      []string{},
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
	if err := normalizeBotSettings(&settings); err != nil {
		return nil, err
	}
	if err := normalizeExpansions(&settings); err != nil {
		return nil, err
	}

	normalized, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	return model.JSONB(normalized), nil
}

func (e *Engine) ValidateRoomConfig(maxPlayers int, rawSettings model.JSONB) error {
	if maxPlayers < minPlayersLimit || maxPlayers > maxPlayersLimit {
		return gameService.ErrInvalidRoomConfig
	}
	var settings carcassonneDTO.RoomSettings
	if err := json.Unmarshal(rawSettings, &settings); err != nil {
		return gameService.ErrInvalidRoomSettings
	}
	if err := normalizeExpansions(&settings); err != nil {
		return err
	}
	if len(settings.Bots) > maxPlayers {
		return gameService.ErrInvalidRoomConfig
	}

	return nil
}

func normalizeBotSettings(settings *carcassonneDTO.RoomSettings) error {
	for i := range settings.Bots {
		difficulty, ok := normalizeBotDifficulty(BotDifficulty(strings.TrimSpace(settings.Bots[i].Difficulty)))
		if !ok {
			return gameService.ErrInvalidRoomSettings
		}
		settings.Bots[i].Difficulty = string(difficulty)
	}
	if settings.Bots == nil {
		settings.Bots = []carcassonneDTO.BotSettings{}
	}
	return nil
}

func normalizeExpansions(settings *carcassonneDTO.RoomSettings) error {
	seen := make(map[string]struct{}, len(settings.Expansions))
	expansions := make([]string, 0, len(settings.Expansions))
	for _, expansion := range settings.Expansions {
		expansion = strings.TrimSpace(expansion)
		switch expansion {
		case "":
			continue
		case ExpansionInnsAndCathedrals:
			if _, ok := seen[expansion]; ok {
				continue
			}
			seen[expansion] = struct{}{}
			expansions = append(expansions, expansion)
		default:
			return gameService.ErrInvalidRoomSettings
		}
	}
	settings.Expansions = expansions
	return nil
}

func (e *Engine) BuildRoomBotParticipants(roomID string, settings model.JSONB, joinedAt time.Time) ([]model.RoomParticipant, error) {
	var roomSettings carcassonneDTO.RoomSettings
	if err := json.Unmarshal(settings, &roomSettings); err != nil {
		return nil, gameService.ErrInvalidRoomSettings
	}
	if err := normalizeBotSettings(&roomSettings); err != nil {
		return nil, err
	}

	bots := make([]model.RoomParticipant, 0, len(roomSettings.Bots))
	for i, bot := range roomSettings.Bots {
		identity := botActorIdentity{
			RoomID: roomID,
			Number: i + 1,
		}
		bots = append(bots, model.RoomParticipant{
			RoomID:        roomID,
			ActorID:       formatBotActorID(identity),
			ActorType:     model.ActorTypeBot,
			DisplayName:   formatBotDisplayName(identity),
			BotDifficulty: bot.Difficulty,
			JoinedAt:      joinedAt,
		})
	}
	return bots, nil
}
