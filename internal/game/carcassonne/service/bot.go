package service

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strconv"
	"strings"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

type BotDifficulty string

const (
	BotDifficultyEasy   BotDifficulty = "easy"
	BotDifficultyMedium BotDifficulty = "medium"
	BotDifficultyHard   BotDifficulty = "hard"
)

type botActorIdentity struct {
	RoomID string
	Number int
}

type botTileOption struct {
	placement autoTilePlacement
	score     float64
}

type botMeepleOption struct {
	placement carcassonneDTO.ValidMeeplePlacement
	score     float64
}

func (e *Engine) BuildBotAction(match *model.Match, actor model.MatchPlayer) (gameService.ApplyActionRequest, error) {
	_, difficulty, ok := parseBotActor(actor)
	if !ok {
		return gameService.ApplyActionRequest{}, gameService.ErrInvalidMatchAction
	}

	var state carcassonneDTO.GameState
	if err := json.Unmarshal(match.GameState, &state); err != nil {
		return gameService.ApplyActionRequest{}, gameService.ErrInvalidMatchAction
	}

	if state.CurrentPlayerID != actor.ActorID {
		return gameService.ApplyActionRequest{}, gameService.ErrNotYourTurn
	}

	switch state.Phase {
	case carcassonneDTO.PhasePlaceTile:
		return e.buildBotPlaceTileAction(match, state, actor.ActorID, difficulty)

	case carcassonneDTO.PhasePlaceMeeple:
		return e.buildBotMeepleAction(match, state, actor.ActorID, difficulty)

	default:
		return gameService.ApplyActionRequest{}, gameService.ErrInvalidMatchAction
	}
}

func (e *Engine) buildBotPlaceTileAction(
	match *model.Match,
	state carcassonneDTO.GameState,
	actorID string,
	difficulty BotDifficulty,
) (gameService.ApplyActionRequest, error) {
	placement, ok := e.chooseBotTilePlacement(state, actorID, difficulty)
	if !ok {
		return gameService.ApplyActionRequest{}, gameService.ErrInvalidMatchAction
	}
	payload, err := marshalActionPayload(carcassonneDTO.PlaceTilePayload{
		RoomID:   match.RoomID,
		X:        placement.X,
		Y:        placement.Y,
		Rotation: placement.Rotation,
	})
	if err != nil {
		return gameService.ApplyActionRequest{}, err
	}
	return gameService.ApplyActionRequest{
		ActorID: actorID,
		Action:  string(carcassonneDTO.ActionPlaceTile),
		Payload: payload,
	}, nil
}

func (e *Engine) buildBotMeepleAction(
	match *model.Match,
	state carcassonneDTO.GameState,
	actorID string,
	difficulty BotDifficulty,
) (gameService.ApplyActionRequest, error) {
	placement, ok := e.chooseBotMeeplePlacement(state, actorID, difficulty)
	if !ok {
		return buildBotSkipMeepleAction(match.RoomID, actorID)
	}
	payload, err := marshalActionPayload(carcassonneDTO.PlaceMeeplePayload{
		RoomID:         match.RoomID,
		TileInstanceID: placement.TileInstanceID,
		ZoneID:         placement.ZoneID,
		Segment:        placement.Segment,
	})
	if err != nil {
		return gameService.ApplyActionRequest{}, err
	}
	return gameService.ApplyActionRequest{
		ActorID: actorID,
		Action:  string(carcassonneDTO.ActionPlaceMeeple),
		Payload: payload,
	}, nil
}

func buildBotSkipMeepleAction(roomID string, actorID string) (gameService.ApplyActionRequest, error) {
	payload, err := marshalActionPayload(carcassonneDTO.SkipMeeplePayload{RoomID: roomID})
	if err != nil {
		return gameService.ApplyActionRequest{}, err
	}
	return gameService.ApplyActionRequest{
		ActorID: actorID,
		Action:  string(carcassonneDTO.ActionSkipMeeple),
		Payload: payload,
	}, nil
}

func normalizeBotDifficulty(difficulty BotDifficulty) (BotDifficulty, bool) {
	switch difficulty {
	case "":
		return BotDifficultyMedium, true
	case BotDifficultyEasy, BotDifficultyMedium, BotDifficultyHard:
		return difficulty, true
	default:
		return "", false
	}
}

func parseBotActor(actor model.MatchPlayer) (botActorIdentity, BotDifficulty, bool) {
	if actor.ActorType != model.ActorTypeBot {
		return botActorIdentity{}, "", false
	}
	if actor.BotDifficulty == "" {
		return botActorIdentity{}, "", false
	}
	difficulty, ok := normalizeBotDifficulty(BotDifficulty(actor.BotDifficulty))
	if !ok {
		return botActorIdentity{}, "", false
	}
	identity, ok := parseBotActorID(actor.ActorID)
	if !ok {
		return botActorIdentity{}, "", false
	}
	return identity, difficulty, true
}

func formatBotActorID(identity botActorIdentity) string {
	return fmt.Sprintf("bot:%s:%d", identity.RoomID, identity.Number)
}

func formatBotDisplayName(identity botActorIdentity) string {
	return fmt.Sprintf("Bot %d", identity.Number)
}

func parseBotActorID(actorID string) (botActorIdentity, bool) {
	kind, rest, ok := strings.Cut(actorID, ":")
	if !ok || kind != "bot" {
		return botActorIdentity{}, false
	}

	roomID, numberRaw, ok := strings.Cut(rest, ":")
	if !ok || roomID == "" {
		return botActorIdentity{}, false
	}
	number, err := strconv.Atoi(numberRaw)
	if err != nil || number <= 0 {
		return botActorIdentity{}, false
	}
	return botActorIdentity{
		RoomID: roomID,
		Number: number,
	}, true
}

func (e *Engine) chooseBotTilePlacement(
	state carcassonneDTO.GameState,
	actorID string,
	difficulty BotDifficulty,
) (autoTilePlacement, bool) {
	options := e.botTileOptions(state, actorID, difficulty)
	if len(options) == 0 {
		return autoTilePlacement{}, false
	}
	if difficulty == BotDifficultyEasy {
		return options[rand.Intn(len(options))].placement, true
	}

	slices.SortFunc(options, func(a, b botTileOption) int {
		if a.score > b.score {
			return -1
		}
		if a.score < b.score {
			return 1
		}
		return compareTilePlacement(a.placement, b.placement)
	})
	return options[0].placement, true
}

func (e *Engine) botTileOptions(
	state carcassonneDTO.GameState,
	actorID string,
	difficulty BotDifficulty,
) []botTileOption {
	placements := e.validTilePlacements(state)
	options := make([]botTileOption, 0)

	for _, placement := range placements {
		for _, rotation := range placement.Rotations {
			tilePlacement := autoTilePlacement{X: placement.X, Y: placement.Y, Rotation: rotation}
			score := 0.0
			if difficulty != BotDifficultyEasy {
				score = e.scoreBotTilePlacement(state, actorID, tilePlacement, difficulty)
			}
			options = append(options, botTileOption{
				placement: tilePlacement,
				score:     score,
			})
		}
	}

	return options
}

func (e *Engine) scoreBotTilePlacement(
	state carcassonneDTO.GameState,
	actorID string,
	placement autoTilePlacement,
	difficulty BotDifficulty,
) float64 {
	simulated := cloneGameState(state)
	if simulated.CurrentTile == nil {
		return math.Inf(-1)
	}

	placedTile := carcassonneDTO.PlacedTile{
		InstanceID: simulated.CurrentTile.InstanceID,
		TileID:     simulated.CurrentTile.TileID,
		X:          placement.X,
		Y:          placement.Y,
		Rotation:   normalizeRotation(placement.Rotation),
		PlacedBy:   actorID,
		TurnNumber: simulated.TurnNumber,
	}
	simulated.Board = append(simulated.Board, placedTile)
	simulated.LastPlacedTile = &placedTile
	simulated.CurrentTile = nil
	simulated.Phase = carcassonneDTO.PhasePlaceMeeple

	before := scoresByActor(state.Players)
	if err := e.scoreCompletedRoads(&simulated); err != nil {
		return math.Inf(-1)
	}
	if err := e.scoreCompletedCities(&simulated); err != nil {
		return math.Inf(-1)
	}
	if err := e.scoreCompletedMonasteries(&simulated); err != nil {
		return math.Inf(-1)
	}
	after := scoresByActor(simulated.Players)

	score := float64(after[actorID] - before[actorID])
	if difficulty == BotDifficultyHard {
		score -= float64(maxOpponentScoreDelta(before, after, actorID)) * 1.5
		score += float64(len(adjacentTiles(state.Board, placement.X, placement.Y))) * 0.2
	}

	meeplePlacements := e.validMeeplePlacements(simulated, actorID)
	score += e.bestBotMeeplePreviewScore(simulated, actorID, meeplePlacements, difficulty) * 0.35
	return score
}

func (e *Engine) chooseBotMeeplePlacement(
	state carcassonneDTO.GameState,
	actorID string,
	difficulty BotDifficulty,
) (carcassonneDTO.ValidMeeplePlacement, bool) {
	placements := e.validMeeplePlacements(state, actorID)
	if len(placements) == 0 {
		return carcassonneDTO.ValidMeeplePlacement{}, false
	}
	if difficulty == BotDifficultyEasy {
		return placements[rand.Intn(len(placements))], true
	}

	options := make([]botMeepleOption, 0, len(placements))
	for _, placement := range placements {
		options = append(options, botMeepleOption{
			placement: placement,
			score:     e.scoreBotMeeplePlacement(state, actorID, placement, difficulty),
		})
	}

	slices.SortFunc(options, func(a, b botMeepleOption) int {
		if a.score > b.score {
			return -1
		}
		if a.score < b.score {
			return 1
		}
		return compareMeeplePlacement(a.placement, b.placement)
	})

	if options[0].score <= 0 {
		return carcassonneDTO.ValidMeeplePlacement{}, false
	}
	return options[0].placement, true
}

func (e *Engine) bestBotMeeplePreviewScore(
	state carcassonneDTO.GameState,
	actorID string,
	placements []carcassonneDTO.ValidMeeplePlacement,
	difficulty BotDifficulty,
) float64 {
	best := 0.0
	for _, placement := range placements {
		score := e.scoreBotMeeplePlacement(state, actorID, placement, difficulty)
		if score > best {
			best = score
		}
	}
	return best
}

func (e *Engine) scoreBotMeeplePlacement(
	state carcassonneDTO.GameState,
	actorID string,
	placement carcassonneDTO.ValidMeeplePlacement,
	difficulty BotDifficulty,
) float64 {
	simulated := cloneGameState(state)
	simulated.Meeples = append(simulated.Meeples, carcassonneDTO.PlacedMeeple{
		TileInstanceID: placement.TileInstanceID,
		ZoneID:         placement.ZoneID,
		ActorID:        actorID,
		FeatureType:    placement.FeatureType,
		Segment:        placement.Segment,
	})
	for i := range simulated.Players {
		if simulated.Players[i].ActorID == actorID {
			simulated.Players[i].MeeplesLeft--
			break
		}
	}

	before := scoresByActor(state.Players)
	if err := e.scoreCompletedRoads(&simulated); err != nil {
		return math.Inf(-1)
	}
	if err := e.scoreCompletedCities(&simulated); err != nil {
		return math.Inf(-1)
	}
	if err := e.scoreCompletedMonasteries(&simulated); err != nil {
		return math.Inf(-1)
	}
	after := scoresByActor(simulated.Players)

	immediateScore := float64(after[actorID] - before[actorID])
	if immediateScore > 0 {
		return immediateScore + 100
	}
	if difficulty == BotDifficultyMedium {
		if placement.FeatureType == carcassonneDTO.ZoneTypeField {
			return -1
		}
		return 0.5
	}

	return e.scoreLongTermMeeplePlacement(state, placement)
}

func (e *Engine) scoreLongTermMeeplePlacement(
	state carcassonneDTO.GameState,
	placement carcassonneDTO.ValidMeeplePlacement,
) float64 {
	switch placement.FeatureType {
	case carcassonneDTO.ZoneTypeCity, carcassonneDTO.ZoneTypeRoad:
		feature, err := e.buildFeature(state, placedZoneRef{
			TileInstanceID: placement.TileInstanceID,
			ZoneID:         placement.ZoneID,
		})
		if err != nil {
			return 0
		}
		tileCount := len(uniqueTileInstanceIDs(feature))
		openPenalty := float64(feature.OpenEdges) * 0.45
		if placement.FeatureType == carcassonneDTO.ZoneTypeCity {
			return float64(tileCount)*1.7 + float64(e.countPennantsForFeature(state, feature))*1.5 - openPenalty
		}
		return float64(tileCount) - openPenalty

	case carcassonneDTO.ZoneTypeMonastery:
		if state.LastPlacedTile == nil {
			return 0
		}
		return float64(1+adjacentTileCount(state.Board, state.LastPlacedTile.X, state.LastPlacedTile.Y)) * 0.8

	case carcassonneDTO.ZoneTypeField:
		graph, err := e.buildFieldGraph(state)
		if err != nil {
			return 0
		}
		roots := graph.rootsForZone(placedZoneRef{
			TileInstanceID: placement.TileInstanceID,
			ZoneID:         placement.ZoneID,
		})
		score := 0.0
		for _, root := range roots {
			completedCities, err := e.fieldCompletedCityCount(state, graph, root)
			if err != nil {
				continue
			}
			score += float64(completedCities) * 3
		}
		return score - 0.25

	default:
		return 0
	}
}

func cloneGameState(state carcassonneDTO.GameState) carcassonneDTO.GameState {
	cloned := state
	cloned.Players = slices.Clone(state.Players)
	cloned.Board = slices.Clone(state.Board)
	cloned.DeckRemaining = slices.Clone(state.DeckRemaining)
	cloned.Meeples = slices.Clone(state.Meeples)
	if state.CurrentTile != nil {
		currentTile := *state.CurrentTile
		cloned.CurrentTile = &currentTile
	}
	if state.LastPlacedTile != nil {
		lastPlacedTile := *state.LastPlacedTile
		cloned.LastPlacedTile = &lastPlacedTile
	}
	return cloned
}

func scoresByActor(players []carcassonneDTO.PlayerState) map[string]int {
	scores := make(map[string]int, len(players))
	for _, player := range players {
		scores[player.ActorID] = player.Score
	}
	return scores
}

func maxOpponentScoreDelta(before, after map[string]int, actorID string) int {
	maxDelta := 0
	for currentActorID, currentScore := range after {
		if currentActorID == actorID {
			continue
		}
		delta := currentScore - before[currentActorID]
		if delta > maxDelta {
			maxDelta = delta
		}
	}
	return maxDelta
}

func compareTilePlacement(a, b autoTilePlacement) int {
	if a.X != b.X {
		return a.X - b.X
	}
	if a.Y != b.Y {
		return a.Y - b.Y
	}
	return a.Rotation - b.Rotation
}

func compareMeeplePlacement(a, b carcassonneDTO.ValidMeeplePlacement) int {
	if a.ZoneID < b.ZoneID {
		return -1
	}
	if a.ZoneID > b.ZoneID {
		return 1
	}
	if a.Segment < b.Segment {
		return -1
	}
	if a.Segment > b.Segment {
		return 1
	}
	return 0
}
