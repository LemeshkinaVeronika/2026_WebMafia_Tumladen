package service

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/google/uuid"
	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
	gameService "github.com/webmafia/tumladan/internal/game/service"
	"github.com/webmafia/tumladan/internal/model"
)

func (e *Engine) resolveTurn(state *carcassonneDTO.GameState) (gameService.ApplyActionResult, error) {
	state.Phase = carcassonneDTO.PhaseResolveTurn
	events := make([]gameService.GameEvent, 0)

	if err := e.scoreCompletedRoadsWithEvents(state, &events); err != nil {
		return gameService.ApplyActionResult{}, err
	}

	if err := e.scoreCompletedCitiesWithEvents(state, &events); err != nil {
		return gameService.ApplyActionResult{}, err
	}

	if err := e.scoreCompletedMonasteriesWithEvents(state, &events); err != nil {
		return gameService.ApplyActionResult{}, err
	}

	result, err := e.completeTurnAndDrawNextTileWithEvents(state, &events)
	if err != nil {
		return gameService.ApplyActionResult{}, err
	}
	result.Events = events
	return result, nil
}

func (e *Engine) scoreCompletedRoads(state *carcassonneDTO.GameState) error {
	return e.scoreCompletedRoadsWithEvents(state, nil)
}

func (e *Engine) scoreCompletedRoadsWithEvents(
	state *carcassonneDTO.GameState,
	events *[]gameService.GameEvent,
) error {
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

		if events != nil {
			contributions := e.completedRoadContributions(*state, feature)
			if scoreContributionTotal(contributions) != score {
				return gameService.ErrInvalidMatchAction
			}
			event, err := e.newFeatureScoredEvent(
				*state,
				carcassonneDTO.ZoneTypeRoad,
				winners,
				score,
				contributions,
				e.meeplesForFeature(*state, feature),
			)
			if err != nil {
				return err
			}
			*events = append(*events, event)
		}

		e.applyScore(state, winners, score)
		e.returnMeeplesForFeature(state, feature)
	}

	return nil
}

func (e *Engine) scoreCompletedMonasteries(state *carcassonneDTO.GameState) error {
	return e.scoreCompletedMonasteriesWithEvents(state, nil)
}

func (e *Engine) scoreCompletedMonasteriesWithEvents(
	state *carcassonneDTO.GameState,
	events *[]gameService.GameEvent,
) error {
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
		if events != nil {
			contributions := []carcassonneDTO.FeatureScoreContribution{
				{
					TileInstanceID: meeple.TileInstanceID,
					ZoneID:         meeple.ZoneID,
					Points:         9,
				},
			}
			event, err := e.newFeatureScoredEvent(
				*state,
				carcassonneDTO.ZoneTypeMonastery,
				[]string{meeple.ActorID},
				9,
				contributions,
				[]carcassonneDTO.PlacedMeeple{meeple},
			)
			if err != nil {
				return err
			}
			*events = append(*events, event)
		}

		e.applyScore(state, []string{meeple.ActorID}, 9)
		e.returnMeeple(state, meeple.TileInstanceID, meeple.ZoneID, meeple.ActorID)
	}

	return nil
}

func (e *Engine) completeTurnAndDrawNextTile(state *carcassonneDTO.GameState) (gameService.ApplyActionResult, error) {
	return e.completeTurnAndDrawNextTileWithEvents(state, nil)
}

func (e *Engine) completeTurnAndDrawNextTileWithEvents(
	state *carcassonneDTO.GameState,
	events *[]gameService.GameEvent,
) (gameService.ApplyActionResult, error) {
	nextTile, deckRemaining := e.drawNextPlaceableTile(state.DeckRemaining, state.Board)
	state.DeckRemaining = deckRemaining
	state.CurrentTile = nextTile
	state.LastPlacedTile = nil

	if nextTile == nil {
		if err := e.scoreFinalFeaturesWithEvents(state, events); err != nil {
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

type finalOccupiedFeature struct {
	featureType carcassonneDTO.ZoneType
	canonical   placedZoneRef
	feature     *feature
}

func (e *Engine) scoreFinalFeatures(state *carcassonneDTO.GameState) error {
	return e.scoreFinalFeaturesWithEvents(state, nil)
}

func (e *Engine) scoreFinalFeaturesWithEvents(
	state *carcassonneDTO.GameState,
	events *[]gameService.GameEvent,
) error {
	features, err := e.finalOccupiedRoadsAndCities(*state)
	if err != nil {
		return err
	}

	for _, featureType := range []carcassonneDTO.ZoneType{
		carcassonneDTO.ZoneTypeRoad,
		carcassonneDTO.ZoneTypeCity,
	} {
		for _, candidate := range features {
			if candidate.featureType != featureType {
				continue
			}

			winners := winningActors(candidate.feature.MeeplesByActor)
			if len(winners) == 0 {
				continue
			}

			score := e.finalFeatureScore(*state, featureType, candidate.feature)
			if events != nil {
				contributions := e.finalFeatureContributions(*state, featureType, candidate.feature, score)
				if scoreContributionTotal(contributions) != score {
					return gameService.ErrInvalidMatchAction
				}
				e.sortScoreContributions(*state, contributions)
				returnedMeeples := e.meeplesForFeature(*state, candidate.feature)
				e.sortMeeples(*state, returnedMeeples)

				event, eventErr := e.newFeatureScoredEventWithDetails(
					*state,
					featureType,
					winners,
					score,
					contributions,
					returnedMeeples,
					featureScoredEventDetails{scoringPhase: carcassonneDTO.ScoringPhaseFinal},
				)
				if eventErr != nil {
					return eventErr
				}
				*events = append(*events, event)
			}

			if score > 0 {
				e.applyScore(state, winners, score)
			}
		}
	}

	if err := e.scoreFinalMonasteriesWithEvents(state, events); err != nil {
		return err
	}

	if err := e.scoreFinalFieldsWithEvents(state, events); err != nil {
		return err
	}

	e.returnAllMeeples(state)
	return nil
}

func (e *Engine) finalOccupiedRoadsAndCities(
	state carcassonneDTO.GameState,
) ([]finalOccupiedFeature, error) {
	seen := make(map[placedZoneRef]struct{})
	result := make([]finalOccupiedFeature, 0)

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
		if !ok || (zone.Type != carcassonneDTO.ZoneTypeRoad && zone.Type != carcassonneDTO.ZoneTypeCity) {
			continue
		}

		feature, err := e.buildFeature(state, placedZoneRef{
			TileInstanceID: meeple.TileInstanceID,
			ZoneID:         meeple.ZoneID,
		})
		if err != nil {
			return nil, err
		}

		identity := canonicalFeatureRef(feature)
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		result = append(result, finalOccupiedFeature{
			featureType: zone.Type,
			canonical:   canonicalFeatureRefOnBoard(state, feature),
			feature:     feature,
		})
	}

	slices.SortFunc(result, func(a, b finalOccupiedFeature) int {
		if a.featureType != b.featureType {
			if a.featureType == carcassonneDTO.ZoneTypeRoad {
				return -1
			}
			return 1
		}
		return compareZoneRefsOnBoard(state.Board, a.canonical, b.canonical)
	})

	return result, nil
}

func (e *Engine) finalFeatureContributions(
	state carcassonneDTO.GameState,
	featureType carcassonneDTO.ZoneType,
	feature *feature,
	score int,
) []carcassonneDTO.FeatureScoreContribution {
	pointsPerTile := 1
	if score == 0 {
		pointsPerTile = 0
	}

	return e.featureScoreContributions(
		state,
		feature,
		pointsPerTile,
		featureType == carcassonneDTO.ZoneTypeCity,
	)
}

func (e *Engine) scoreFinalMonasteriesWithEvents(
	state *carcassonneDTO.GameState,
	events *[]gameService.GameEvent,
) error {
	monasteries := make([]carcassonneDTO.PlacedMeeple, 0)
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
		if ok && zone.Type == carcassonneDTO.ZoneTypeMonastery {
			monasteries = append(monasteries, meeple)
		}
	}

	e.sortMeeples(*state, monasteries)
	for _, meeple := range monasteries {
		tile := placedTileByInstanceID(state.Board, meeple.TileInstanceID)
		if tile == nil {
			continue
		}
		score := 1 + adjacentTileCount(state.Board, tile.X, tile.Y)
		if events != nil {
			contributions := []carcassonneDTO.FeatureScoreContribution{{
				TileInstanceID: meeple.TileInstanceID,
				ZoneID:         meeple.ZoneID,
				Points:         score,
			}}
			event, err := e.newFeatureScoredEventWithDetails(
				*state,
				carcassonneDTO.ZoneTypeMonastery,
				[]string{meeple.ActorID},
				score,
				contributions,
				[]carcassonneDTO.PlacedMeeple{meeple},
				featureScoredEventDetails{scoringPhase: carcassonneDTO.ScoringPhaseFinal},
			)
			if err != nil {
				return err
			}
			*events = append(*events, event)
		}
		e.applyScore(state, []string{meeple.ActorID}, score)
	}

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
	return e.scoreFinalFieldsWithEvents(state, nil)
}

type finalOccupiedField struct {
	root      placedFieldSegmentRef
	canonical placedFieldSegmentRef
}

func (e *Engine) scoreFinalFieldsWithEvents(
	state *carcassonneDTO.GameState,
	events *[]gameService.GameEvent,
) error {
	graph, err := e.buildFieldGraph(*state)
	if err != nil {
		return err
	}

	fields := make([]finalOccupiedField, 0, len(graph.meeplesByRoot))
	for root := range graph.meeplesByRoot {
		fields = append(fields, finalOccupiedField{
			root:      root,
			canonical: canonicalFieldSegmentRef(*state, graph, root),
		})
	}
	slices.SortFunc(fields, func(a, b finalOccupiedField) int {
		return compareFieldSegmentRefsOnBoard(state.Board, a.canonical, b.canonical)
	})

	for _, field := range fields {
		meeplesByActor := graph.meeplesByRoot[field.root]
		winners := winningActors(meeplesByActor)
		if len(winners) == 0 {
			continue
		}

		completedCities, err := e.fieldCompletedCities(*state, graph, field.root)
		if err != nil {
			return err
		}
		score := len(completedCities) * 3
		returnedMeeples := e.meeplesForField(*state, graph, field.root)

		if events != nil {
			contributingCities := e.finalFieldContributingCities(*state, completedCities)
			scoreMarkers := e.finalFieldScoreMarkers(returnedMeeples, winners, score)
			event, eventErr := e.newFeatureScoredEventWithDetails(
				*state,
				carcassonneDTO.ZoneTypeField,
				winners,
				score,
				[]carcassonneDTO.FeatureScoreContribution{},
				returnedMeeples,
				featureScoredEventDetails{
					scoringPhase:       carcassonneDTO.ScoringPhaseFinal,
					contributingCities: contributingCities,
					scoreMarkers:       scoreMarkers,
				},
			)
			if eventErr != nil {
				return eventErr
			}
			*events = append(*events, event)
		}

		if score > 0 {
			e.applyScore(state, winners, score)
		}
	}

	return nil
}

func canonicalFeatureRefOnBoard(state carcassonneDTO.GameState, scoredFeature *feature) placedZoneRef {
	if scoredFeature == nil || len(scoredFeature.Zones) == 0 {
		return placedZoneRef{}
	}

	zones := slices.Clone(scoredFeature.Zones)
	slices.SortFunc(zones, func(a, b placedZoneRef) int {
		return compareZoneRefsOnBoard(state.Board, a, b)
	})
	return zones[0]
}

func canonicalFieldSegmentRef(
	state carcassonneDTO.GameState,
	graph *fieldGraph,
	root placedFieldSegmentRef,
) placedFieldSegmentRef {
	if graph == nil || len(graph.segmentsByRoot[root]) == 0 {
		return placedFieldSegmentRef{}
	}

	segments := slices.Clone(graph.segmentsByRoot[root])
	slices.SortFunc(segments, func(a, b placedFieldSegmentRef) int {
		return compareFieldSegmentRefsOnBoard(state.Board, a, b)
	})
	return segments[0]
}

func compareZoneRefsOnBoard(board []carcassonneDTO.PlacedTile, a, b placedZoneRef) int {
	if result := compareTileInstanceIDsOnBoard(board, a.TileInstanceID, b.TileInstanceID); result != 0 {
		return result
	}
	if a.ZoneID < b.ZoneID {
		return -1
	}
	if a.ZoneID > b.ZoneID {
		return 1
	}
	return 0
}

func compareFieldSegmentRefsOnBoard(
	board []carcassonneDTO.PlacedTile,
	a, b placedFieldSegmentRef,
) int {
	if result := compareTileInstanceIDsOnBoard(board, a.TileInstanceID, b.TileInstanceID); result != 0 {
		return result
	}
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

func compareTileInstanceIDsOnBoard(board []carcassonneDTO.PlacedTile, a, b string) int {
	if a == b {
		return 0
	}

	tileA := placedTileByInstanceID(board, a)
	tileB := placedTileByInstanceID(board, b)
	if tileA != nil && tileB != nil {
		if tileA.Y < tileB.Y {
			return -1
		}
		if tileA.Y > tileB.Y {
			return 1
		}
		if tileA.X < tileB.X {
			return -1
		}
		if tileA.X > tileB.X {
			return 1
		}
	} else if tileA != nil {
		return -1
	} else if tileB != nil {
		return 1
	}

	if a < b {
		return -1
	}
	return 1
}

func (e *Engine) sortScoreContributions(
	state carcassonneDTO.GameState,
	contributions []carcassonneDTO.FeatureScoreContribution,
) {
	slices.SortFunc(contributions, func(a, b carcassonneDTO.FeatureScoreContribution) int {
		return compareZoneRefsOnBoard(state.Board, placedZoneRef{
			TileInstanceID: a.TileInstanceID,
			ZoneID:         a.ZoneID,
		}, placedZoneRef{
			TileInstanceID: b.TileInstanceID,
			ZoneID:         b.ZoneID,
		})
	})
}

func (e *Engine) sortMeeples(state carcassonneDTO.GameState, meeples []carcassonneDTO.PlacedMeeple) {
	slices.SortFunc(meeples, func(a, b carcassonneDTO.PlacedMeeple) int {
		if result := compareZoneRefsOnBoard(state.Board, placedZoneRef{
			TileInstanceID: a.TileInstanceID,
			ZoneID:         a.ZoneID,
		}, placedZoneRef{
			TileInstanceID: b.TileInstanceID,
			ZoneID:         b.ZoneID,
		}); result != 0 {
			return result
		}
		if a.ActorID < b.ActorID {
			return -1
		}
		if a.ActorID > b.ActorID {
			return 1
		}
		if a.MeepleType < b.MeepleType {
			return -1
		}
		if a.MeepleType > b.MeepleType {
			return 1
		}
		if a.Segment < b.Segment {
			return -1
		}
		if a.Segment > b.Segment {
			return 1
		}
		return 0
	})
}

func (e *Engine) meeplesForField(
	state carcassonneDTO.GameState,
	graph *fieldGraph,
	root placedFieldSegmentRef,
) []carcassonneDTO.PlacedMeeple {
	meeples := make([]carcassonneDTO.PlacedMeeple, 0)
	for _, meeple := range state.Meeples {
		zoneRef := placedZoneRef{TileInstanceID: meeple.TileInstanceID, ZoneID: meeple.ZoneID}
		if slices.Contains(graph.rootsForZone(zoneRef), root) {
			meeples = append(meeples, meeple)
		}
	}
	e.sortMeeples(state, meeples)
	return meeples
}

func (e *Engine) finalFieldContributingCities(
	state carcassonneDTO.GameState,
	completedCities []fieldCompletedCity,
) []carcassonneDTO.FeatureScoreContributingCity {
	result := make([]carcassonneDTO.FeatureScoreContributingCity, 0, len(completedCities))
	for _, completedCity := range completedCities {
		anchor := canonicalFeatureRefOnBoard(state, completedCity.feature)
		tileInstanceIDs := uniqueTileInstanceIDs(completedCity.feature)
		slices.SortFunc(tileInstanceIDs, func(a, b string) int {
			return compareTileInstanceIDsOnBoard(state.Board, a, b)
		})
		zoneRefs := slices.Clone(completedCity.feature.Zones)
		slices.SortFunc(zoneRefs, func(a, b placedZoneRef) int {
			return compareZoneRefsOnBoard(state.Board, a, b)
		})
		zones := make([]carcassonneDTO.FeatureScoreZoneRef, 0, len(zoneRefs))
		for _, zoneRef := range zoneRefs {
			zones = append(zones, carcassonneDTO.FeatureScoreZoneRef{
				TileInstanceID: zoneRef.TileInstanceID,
				ZoneID:         zoneRef.ZoneID,
			})
		}
		result = append(result, carcassonneDTO.FeatureScoreContributingCity{
			AnchorTileInstanceID: anchor.TileInstanceID,
			AnchorZoneID:         anchor.ZoneID,
			TileInstanceIDs:      tileInstanceIDs,
			Zones:                zones,
		})
	}
	slices.SortFunc(result, func(a, b carcassonneDTO.FeatureScoreContributingCity) int {
		return compareZoneRefsOnBoard(state.Board, placedZoneRef{
			TileInstanceID: a.AnchorTileInstanceID,
			ZoneID:         a.AnchorZoneID,
		}, placedZoneRef{
			TileInstanceID: b.AnchorTileInstanceID,
			ZoneID:         b.AnchorZoneID,
		})
	})
	return result
}

func (e *Engine) finalFieldScoreMarkers(
	returnedMeeples []carcassonneDTO.PlacedMeeple,
	winners []string,
	score int,
) []carcassonneDTO.FeatureScoreMarker {
	markers := make([]carcassonneDTO.FeatureScoreMarker, 0, len(winners))
	for _, actorID := range winners {
		for _, meeple := range returnedMeeples {
			if meeple.ActorID != actorID {
				continue
			}
			markers = append(markers, carcassonneDTO.FeatureScoreMarker{
				ActorID:        actorID,
				TileInstanceID: meeple.TileInstanceID,
				ZoneID:         meeple.ZoneID,
				Points:         score,
			})
			break
		}
	}
	return markers
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

func (e *Engine) meeplesForFeature(
	state carcassonneDTO.GameState,
	feature *feature,
) []carcassonneDTO.PlacedMeeple {
	if feature == nil {
		return nil
	}

	meeples := make([]carcassonneDTO.PlacedMeeple, 0)
	for _, meeple := range state.Meeples {
		if slices.ContainsFunc(feature.Zones, func(ref placedZoneRef) bool {
			return ref.TileInstanceID == meeple.TileInstanceID && ref.ZoneID == meeple.ZoneID
		}) {
			meeples = append(meeples, meeple)
		}
	}
	return meeples
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
	slices.Sort(winners)

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

func (e *Engine) completedRoadContributions(
	state carcassonneDTO.GameState,
	feature *feature,
) []carcassonneDTO.FeatureScoreContribution {
	pointsPerTile := 1
	if e.featureHasInn(state, feature) {
		pointsPerTile = 2
	}
	return e.featureScoreContributions(state, feature, pointsPerTile, false)
}

func (e *Engine) completedCityContributions(
	state carcassonneDTO.GameState,
	feature *feature,
) []carcassonneDTO.FeatureScoreContribution {
	pointsPerTile := 2
	if e.featureHasCathedral(state, feature) {
		pointsPerTile = 3
	}
	return e.featureScoreContributions(state, feature, pointsPerTile, true)
}

func (e *Engine) featureScoreContributions(
	state carcassonneDTO.GameState,
	feature *feature,
	pointsPerTile int,
	includePennants bool,
) []carcassonneDTO.FeatureScoreContribution {
	if feature == nil {
		return nil
	}

	contributions := make([]carcassonneDTO.FeatureScoreContribution, 0, len(feature.Zones))
	indexByTile := make(map[string]int, len(feature.Zones))
	seenZones := make(map[placedZoneRef]struct{}, len(feature.Zones))

	for _, ref := range feature.Zones {
		if _, ok := seenZones[ref]; ok {
			continue
		}
		seenZones[ref] = struct{}{}

		index, ok := indexByTile[ref.TileInstanceID]
		if !ok {
			index = len(contributions)
			indexByTile[ref.TileInstanceID] = index
			contributions = append(contributions, carcassonneDTO.FeatureScoreContribution{
				TileInstanceID: ref.TileInstanceID,
				ZoneID:         ref.ZoneID,
				Points:         pointsPerTile,
			})
		}

		if !includePennants {
			continue
		}
		placedZone, err := e.getPlacedZone(state, ref)
		if err == nil && placedZone.Zone.Type == carcassonneDTO.ZoneTypeCity && placedZone.Zone.HasPennant {
			contributions[index].Points += pointsPerTile
		}
	}

	return contributions
}

func scoreContributionTotal(contributions []carcassonneDTO.FeatureScoreContribution) int {
	total := 0
	for _, contribution := range contributions {
		total += contribution.Points
	}
	return total
}

func (e *Engine) newFeatureScoredEvent(
	state carcassonneDTO.GameState,
	featureType carcassonneDTO.ZoneType,
	winners []string,
	totalPoints int,
	contributions []carcassonneDTO.FeatureScoreContribution,
	returnedMeeples []carcassonneDTO.PlacedMeeple,
) (gameService.GameEvent, error) {
	return e.newFeatureScoredEventWithDetails(
		state,
		featureType,
		winners,
		totalPoints,
		contributions,
		returnedMeeples,
		featureScoredEventDetails{scoringPhase: carcassonneDTO.ScoringPhaseTurn},
	)
}

type featureScoredEventDetails struct {
	scoringPhase       carcassonneDTO.ScoringPhase
	contributingCities []carcassonneDTO.FeatureScoreContributingCity
	scoreMarkers       []carcassonneDTO.FeatureScoreMarker
}

func (e *Engine) newFeatureScoredEventWithDetails(
	state carcassonneDTO.GameState,
	featureType carcassonneDTO.ZoneType,
	winners []string,
	totalPoints int,
	contributions []carcassonneDTO.FeatureScoreContribution,
	returnedMeeples []carcassonneDTO.PlacedMeeple,
	details featureScoredEventDetails,
) (gameService.GameEvent, error) {
	anchorTileInstanceID := ""
	if len(details.scoreMarkers) > 0 {
		anchorTileInstanceID = details.scoreMarkers[0].TileInstanceID
	} else if featureType == carcassonneDTO.ZoneTypeMonastery && len(contributions) > 0 {
		anchorTileInstanceID = contributions[0].TileInstanceID
	} else if details.scoringPhase != carcassonneDTO.ScoringPhaseFinal && state.LastPlacedTile != nil {
		anchorTileInstanceID = state.LastPlacedTile.InstanceID
	} else if len(contributions) > 0 {
		anchorTileInstanceID = contributions[0].TileInstanceID
	}

	awards := make([]carcassonneDTO.FeatureScoreAward, 0, len(winners))
	for _, actorID := range winners {
		awards = append(awards, carcassonneDTO.FeatureScoreAward{
			ActorID: actorID,
			Points:  totalPoints,
		})
	}

	payload, err := json.Marshal(carcassonneDTO.FeatureScoredEventPayload{
		TurnNumber:           state.TurnNumber,
		FeatureType:          featureType,
		ScoringPhase:         details.scoringPhase,
		AnchorTileInstanceID: anchorTileInstanceID,
		Contributions:        contributions,
		TotalPoints:          totalPoints,
		Awards:               awards,
		ReturnedMeeples:      returnedMeeples,
		ContributingCities:   details.contributingCities,
		ScoreMarkers:         details.scoreMarkers,
	})
	if err != nil {
		return gameService.GameEvent{}, err
	}

	return gameService.GameEvent{
		ID:      uuid.NewString(),
		Type:    carcassonneDTO.EventTypeFeatureScored,
		Payload: payload,
	}, nil
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
	return e.scoreCompletedCitiesWithEvents(state, nil)
}

func (e *Engine) scoreCompletedCitiesWithEvents(
	state *carcassonneDTO.GameState,
	events *[]gameService.GameEvent,
) error {
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

		if events != nil {
			contributions := e.completedCityContributions(*state, feature)
			if scoreContributionTotal(contributions) != score {
				return gameService.ErrInvalidMatchAction
			}
			event, err := e.newFeatureScoredEvent(
				*state,
				carcassonneDTO.ZoneTypeCity,
				winners,
				score,
				contributions,
				e.meeplesForFeature(*state, feature),
			)
			if err != nil {
				return err
			}
			*events = append(*events, event)
		}

		e.applyScore(state, winners, score)
		e.returnMeeplesForFeature(state, feature)
	}

	return nil
}
