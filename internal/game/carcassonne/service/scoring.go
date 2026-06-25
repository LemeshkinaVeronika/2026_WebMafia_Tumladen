package service

import (
	"encoding/json"
	"slices"
	"time"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

func (e *Engine) resolveTurn(state *carcassonneDTO.GameState) (gameService.ApplyActionResult, error) {
	state.Phase = carcassonneDTO.PhaseResolveTurn

	if err := e.scoreCompletedRoads(state); err != nil {
		return gameService.ApplyActionResult{}, err
	}

	if err := e.scoreCompletedCities(state); err != nil {
		return gameService.ApplyActionResult{}, err
	}

	if err := e.scoreCompletedMonasteries(state); err != nil {
		return gameService.ApplyActionResult{}, err
	}

	return e.completeTurnAndDrawNextTile(state)
}

func (e *Engine) scoreCompletedRoads(state *carcassonneDTO.GameState) error {
	startZones := e.lastPlacedZoneRefsByType(*state, carcassonneDTO.ZoneTypeRoad)
	if len(startZones) == 0 {
		return nil
	}

	visited := make(map[placedZoneRef]struct{}, len(startZones))

	for _, start := range startZones {
		if _, ok := visited[start]; ok {
			continue
		}

		feature, err := e.buildFeature(*state, start)
		if err != nil {
			return err
		}

		for _, zoneRef := range feature.Zones {
			visited[zoneRef] = struct{}{}
		}

		if feature.OpenEdges != 0 {
			continue
		}

		winners := winningActors(feature.MeeplesByActor)
		if len(winners) == 0 {
			continue
		}

		score := e.completedRoadScore(*state, feature)
		if score <= 0 {
			continue
		}

		e.applyScore(state, winners, score)
		e.returnMeeplesForFeature(state, feature)
	}

	return nil
}

func (e *Engine) scoreCompletedMonasteries(state *carcassonneDTO.GameState) error {
	completed := make([]carcassonneDTO.PlacedMeeple, 0)

	for _, meeple := range state.Meeples {
		tile := placedTileByInstanceID(state.Board, meeple.TileInstanceID)
		if tile == nil {
			continue
		}

		def, ok := e.catalog.Get(tile.TileID)
		if !ok {
			continue
		}

		zone, ok := findZone(def, meeple.ZoneID)
		if !ok || zone.Type != carcassonneDTO.ZoneTypeMonastery {
			continue
		}

		if !isMonasteryComplete(state.Board, tile.X, tile.Y) {
			continue
		}

		completed = append(completed, meeple)
	}

	for _, meeple := range completed {
		e.applyScore(state, []string{meeple.ActorID}, 9)
		e.returnMeeple(state, meeple.TileInstanceID, meeple.ZoneID, meeple.ActorID)
	}

	return nil
}

func (e *Engine) completeTurnAndDrawNextTile(state *carcassonneDTO.GameState) (gameService.ApplyActionResult, error) {
	nextTile, deckRemaining := e.drawNextPlaceableTile(state.DeckRemaining, state.Board)
	state.DeckRemaining = deckRemaining
	state.CurrentTile = nextTile
	state.LastPlacedTile = nil

	if nextTile == nil {
		if err := e.scoreFinalFeatures(state); err != nil {
			return gameService.ApplyActionResult{}, err
		}

		state.Phase = carcassonneDTO.PhaseFinished
		state.CurrentPlayerID = ""

		result, err := json.Marshal(matchResult(state.Players))
		if err != nil {
			return gameService.ApplyActionResult{}, err
		}

		jsonResult := model.JSONB(result)
		return marshalActionResult(*state, model.MatchStatusFinished, &jsonResult)
	}

	if len(state.Players) > 0 {
		currentPlayerIndex := slices.IndexFunc(state.Players, func(player carcassonneDTO.PlayerState) bool {
			return player.ActorID == state.CurrentPlayerID
		})
		if currentPlayerIndex < 0 {
			currentPlayerIndex = 0
		} else {
			currentPlayerIndex = (currentPlayerIndex + 1) % len(state.Players)
		}
		state.CurrentPlayerID = state.Players[currentPlayerIndex].ActorID
	}

	state.TurnNumber++
	state.TurnStartedAt = time.Now().UTC().Format(time.RFC3339)
	state.Phase = carcassonneDTO.PhasePlaceTile

	return marshalActionResult(*state, model.MatchStatusActive, nil)
}

func matchResult(players []carcassonneDTO.PlayerState) carcassonneDTO.MatchResult {
	finalScores := make([]carcassonneDTO.FinalScore, 0, len(players))
	maxScore := 0

	for i, player := range players {
		finalScores = append(finalScores, carcassonneDTO.FinalScore{
			ActorID: player.ActorID,
			Score:   player.Score,
		})
		if i == 0 || player.Score > maxScore {
			maxScore = player.Score
		}
	}

	winners := make([]string, 0)
	for _, player := range players {
		if player.Score == maxScore {
			winners = append(winners, player.ActorID)
		}
	}

	return carcassonneDTO.MatchResult{
		Winners:     winners,
		FinalScores: finalScores,
	}
}

func (e *Engine) drawNextPlaceableTile(
	deck []carcassonneDTO.TileInstance,
	board []carcassonneDTO.PlacedTile,
) (*carcassonneDTO.TileInstance, []carcassonneDTO.TileInstance) {
	skipped := make([]carcassonneDTO.TileInstance, 0)

	for len(deck) > 0 {
		tile, rest := drawTopTile(deck)
		deck = rest
		if tile == nil {
			break
		}
		if e.hasAnyTilePlacement(*tile, board) {
			deck = append(deck, skipped...)
			return tile, deck
		}
		skipped = append(skipped, *tile)
	}

	deck = append(deck, skipped...)
	return nil, deck
}

func (e *Engine) hasAnyTilePlacement(tile carcassonneDTO.TileInstance, board []carcassonneDTO.PlacedTile) bool {
	candidates := make(map[[2]int]struct{}, len(board)*4)
	for _, placed := range board {
		for _, coordinate := range [][2]int{
			{placed.X, placed.Y - 1},
			{placed.X + 1, placed.Y},
			{placed.X, placed.Y + 1},
			{placed.X - 1, placed.Y},
		} {
			if tileAt(board, coordinate[0], coordinate[1]) == nil {
				candidates[coordinate] = struct{}{}
			}
		}
	}

	for coordinate := range candidates {
		for _, rotation := range []int{0, 90, 180, 270} {
			if e.canPlaceTile(tile, coordinate[0], coordinate[1], rotation, board) {
				return true
			}
		}
	}

	return false
}

func (e *Engine) scoreFinalFeatures(state *carcassonneDTO.GameState) error {
	scoredFeatures := make(map[placedZoneRef]struct{})

	for _, meeple := range state.Meeples {
		tile := placedTileByInstanceID(state.Board, meeple.TileInstanceID)
		if tile == nil {
			continue
		}

		def, ok := e.catalog.Get(tile.TileID)
		if !ok {
			continue
		}

		zone, ok := findZone(def, meeple.ZoneID)
		if !ok {
			continue
		}

		switch zone.Type {
		case carcassonneDTO.ZoneTypeRoad, carcassonneDTO.ZoneTypeCity:
			start := placedZoneRef{
				TileInstanceID: meeple.TileInstanceID,
				ZoneID:         meeple.ZoneID,
			}
			feature, err := e.buildFeature(*state, start)
			if err != nil {
				return err
			}

			canonical := canonicalFeatureRef(feature)
			if _, ok := scoredFeatures[canonical]; ok {
				continue
			}
			scoredFeatures[canonical] = struct{}{}

			winners := winningActors(feature.MeeplesByActor)
			if len(winners) == 0 {
				continue
			}

			score := e.finalFeatureScore(*state, zone.Type, feature)
			if score > 0 {
				e.applyScore(state, winners, score)
			}

		case carcassonneDTO.ZoneTypeMonastery:
			score := 1 + adjacentTileCount(state.Board, tile.X, tile.Y)
			e.applyScore(state, []string{meeple.ActorID}, score)
		}
	}

	if err := e.scoreFinalFields(state); err != nil {
		return err
	}

	e.returnAllMeeples(state)
	return nil
}

func (e *Engine) finalFeatureScore(
	state carcassonneDTO.GameState,
	zoneType carcassonneDTO.ZoneType,
	feature *feature,
) int {
	switch zoneType {
	case carcassonneDTO.ZoneTypeRoad:
		if e.featureHasInn(state, feature) {
			return 0
		}
		return len(uniqueTileInstanceIDs(feature))
	case carcassonneDTO.ZoneTypeCity:
		if e.featureHasCathedral(state, feature) {
			return 0
		}
		tileCount := len(uniqueTileInstanceIDs(feature))
		pennantCount := e.countPennantsForFeature(state, feature)
		return tileCount + pennantCount
	default:
		return 0
	}
}

func (e *Engine) scoreFinalFields(state *carcassonneDTO.GameState) error {
	graph, err := e.buildFieldGraph(*state)
	if err != nil {
		return err
	}

	for root, meeplesByActor := range graph.meeplesByRoot {
		winners := winningActors(meeplesByActor)
		if len(winners) == 0 {
			continue
		}

		completedCities, err := e.fieldCompletedCityCount(*state, graph, root)
		if err != nil {
			return err
		}
		if completedCities == 0 {
			continue
		}

		e.applyScore(state, winners, completedCities*3)
	}

	return nil
}

func (e *Engine) applyScore(state *carcassonneDTO.GameState, actorIDs []string, score int) {
	for i := range state.Players {
		if slices.Contains(actorIDs, state.Players[i].ActorID) {
			state.Players[i].Score += score
		}
	}
}

func (e *Engine) returnAllMeeples(state *carcassonneDTO.GameState) {
	for _, meeple := range state.Meeples {
		incrementMeeple(state.Players, meeple.ActorID, meeple.MeepleType)
	}

	state.Meeples = []carcassonneDTO.PlacedMeeple{}
}

func (e *Engine) returnMeeplesForFeature(state *carcassonneDTO.GameState, feature *feature) {
	if feature == nil {
		return
	}

	remaining := state.Meeples[:0]

	for _, meeple := range state.Meeples {
		inFeature := slices.ContainsFunc(feature.Zones, func(ref placedZoneRef) bool {
			return ref.TileInstanceID == meeple.TileInstanceID && ref.ZoneID == meeple.ZoneID
		})

		if inFeature {
			incrementMeeple(state.Players, meeple.ActorID, meeple.MeepleType)
			continue
		}

		remaining = append(remaining, meeple)
	}

	state.Meeples = remaining
}

func (e *Engine) returnMeeple(state *carcassonneDTO.GameState, tileInstanceID, zoneID, actorID string) {
	remaining := state.Meeples[:0]

	for _, meeple := range state.Meeples {
		if meeple.TileInstanceID == tileInstanceID && meeple.ZoneID == zoneID && meeple.ActorID == actorID {
			incrementMeeple(state.Players, actorID, meeple.MeepleType)
			continue
		}

		remaining = append(remaining, meeple)
	}

	state.Meeples = remaining
}

func winningActors(meeplesByActor map[string]int) []string {
	if len(meeplesByActor) == 0 {
		return nil
	}

	maxCount := 0
	for _, count := range meeplesByActor {
		if count > maxCount {
			maxCount = count
		}
	}

	if maxCount == 0 {
		return nil
	}

	winners := make([]string, 0)
	for actorID, count := range meeplesByActor {
		if count == maxCount {
			winners = append(winners, actorID)
		}
	}

	return winners
}

func meepleWeight(meeple carcassonneDTO.PlacedMeeple) int {
	if normalizeMeepleType(meeple.MeepleType) == carcassonneDTO.MeepleTypeBig {
		return 2
	}
	return 1
}

func incrementMeeple(players []carcassonneDTO.PlayerState, actorID string, meepleType carcassonneDTO.MeepleType) {
	for i := range players {
		if players[i].ActorID != actorID {
			continue
		}
		switch normalizeMeepleType(meepleType) {
		case carcassonneDTO.MeepleTypeBig:
			players[i].BigMeeplesLeft++
		default:
			players[i].MeeplesLeft++
		}
		return
	}
}

func placedTileByInstanceID(board []carcassonneDTO.PlacedTile, instanceID string) *carcassonneDTO.PlacedTile {
	for i := range board {
		if board[i].InstanceID == instanceID {
			return &board[i]
		}
	}
	return nil
}

func isMonasteryComplete(board []carcassonneDTO.PlacedTile, x, y int) bool {
	return adjacentTileCount(board, x, y) == 8
}

func adjacentTileCount(board []carcassonneDTO.PlacedTile, x, y int) int {
	coordinates := [][2]int{
		{x - 1, y - 1},
		{x, y - 1},
		{x + 1, y - 1},
		{x - 1, y},
		{x + 1, y},
		{x - 1, y + 1},
		{x, y + 1},
		{x + 1, y + 1},
	}

	count := 0
	for _, coordinate := range coordinates {
		if tileAt(board, coordinate[0], coordinate[1]) != nil {
			count++
		}
	}

	return count
}

func uniqueTileInstanceIDs(feature *feature) []string {
	if feature == nil {
		return nil
	}

	seen := make(map[string]struct{}, len(feature.Zones))
	result := make([]string, 0, len(feature.Zones))

	for _, zone := range feature.Zones {
		if _, ok := seen[zone.TileInstanceID]; ok {
			continue
		}
		seen[zone.TileInstanceID] = struct{}{}
		result = append(result, zone.TileInstanceID)
	}

	return result
}

func (e *Engine) lastPlacedZoneRefsByType(
	state carcassonneDTO.GameState,
	zoneType carcassonneDTO.ZoneType,
) []placedZoneRef {
	if state.LastPlacedTile == nil {
		return nil
	}

	def, ok := e.catalog.Get(state.LastPlacedTile.TileID)
	if !ok {
		return nil
	}

	result := make([]placedZoneRef, 0, len(def.Zones))
	for _, zone := range def.Zones {
		if zone.Type != zoneType {
			continue
		}
		result = append(result, placedZoneRef{
			TileInstanceID: state.LastPlacedTile.InstanceID,
			ZoneID:         zone.ZoneID,
		})
	}

	return result
}

func (e *Engine) countPennantsForFeature(state carcassonneDTO.GameState, feature *feature) int {
	if feature == nil {
		return 0
	}

	seen := make(map[placedZoneRef]struct{}, len(feature.Zones))
	count := 0

	for _, ref := range feature.Zones {
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}

		placedZone, err := e.getPlacedZone(state, ref)
		if err != nil {
			continue
		}
		if placedZone.Zone.Type == carcassonneDTO.ZoneTypeCity && placedZone.Zone.HasPennant {
			count++
		}
	}

	return count
}

func (e *Engine) completedRoadScore(state carcassonneDTO.GameState, feature *feature) int {
	tileCount := len(uniqueTileInstanceIDs(feature))
	if e.featureHasInn(state, feature) {
		return tileCount * 2
	}
	return tileCount
}

func (e *Engine) completedCityScore(
	state carcassonneDTO.GameState,
	feature *feature,
	tileCount int,
	pennantCount int,
) int {
	if e.featureHasCathedral(state, feature) {
		return tileCount*3 + pennantCount*3
	}
	return tileCount*2 + pennantCount*2
}

func (e *Engine) featureHasInn(state carcassonneDTO.GameState, feature *feature) bool {
	if feature == nil {
		return false
	}

	for _, ref := range feature.Zones {
		placedZone, err := e.getPlacedZone(state, ref)
		if err != nil {
			continue
		}
		if placedZone.Zone.Type == carcassonneDTO.ZoneTypeRoad && placedZone.Zone.HasInn {
			return true
		}
	}
	return false
}

func (e *Engine) featureHasCathedral(state carcassonneDTO.GameState, feature *feature) bool {
	if feature == nil {
		return false
	}

	tileInstanceIDs := make(map[string]struct{}, len(feature.Zones))
	for _, ref := range feature.Zones {
		tileInstanceIDs[ref.TileInstanceID] = struct{}{}
	}

	for tileInstanceID := range tileInstanceIDs {
		tile := placedTileByInstanceID(state.Board, tileInstanceID)
		if tile == nil {
			continue
		}
		def, ok := e.catalog.Get(tile.TileID)
		if !ok {
			continue
		}
		for _, zone := range def.Zones {
			if zone.Type == carcassonneDTO.ZoneTypeCathedral {
				return true
			}
		}
	}
	return false
}

func (e *Engine) scoreCompletedCities(state *carcassonneDTO.GameState) error {
	startZones := e.lastPlacedZoneRefsByType(*state, carcassonneDTO.ZoneTypeCity)
	if len(startZones) == 0 {
		return nil
	}

	visited := make(map[placedZoneRef]struct{}, len(startZones))

	for _, start := range startZones {
		if _, ok := visited[start]; ok {
			continue
		}

		feature, err := e.buildFeature(*state, start)
		if err != nil {
			return err
		}

		for _, zoneRef := range feature.Zones {
			visited[zoneRef] = struct{}{}
		}

		if feature.OpenEdges != 0 {
			continue
		}

		winners := winningActors(feature.MeeplesByActor)
		if len(winners) == 0 {
			continue
		}

		tileCount := len(uniqueTileInstanceIDs(feature))
		pennantCount := e.countPennantsForFeature(*state, feature)
		score := e.completedCityScore(*state, feature, tileCount, pennantCount)
		if score <= 0 {
			continue
		}

		e.applyScore(state, winners, score)
		e.returnMeeplesForFeature(state, feature)
	}

	return nil
}
