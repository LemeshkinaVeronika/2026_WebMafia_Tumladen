package service

import (
	"fmt"
	"slices"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

type feature struct {
	Type           carcassonneDTO.ZoneType
	Zones          []placedZoneRef
	OpenEdges      int
	MeeplesByActor map[string]int
}

func (e *Engine) connectedNeighborZones(state carcassonneDTO.GameState, zone *placedZone) ([]placedZoneRef, int, error) {
	neighbors := make([]placedZoneRef, 0)
	openEdges := 0

	for _, segment := range zone.Segments {
		match, ok := borderNeighbor(segment)
		if !ok {
			continue
		}

		neighborX := zone.Tile.X + match.DX
		neighborY := zone.Tile.Y + match.DY

		neighborTile := tileAt(state.Board, neighborX, neighborY)
		if neighborTile == nil {
			openEdges++
			continue
		}

		neighborRef, err := e.findNeighborZoneBySegment(
			state,
			neighborX,
			neighborY,
			zone.Zone.Type,
			match.NeighborSegment,
		)
		if err != nil {
			return nil, 0, err
		}
		if neighborRef == nil {
			openEdges++
			continue
		}

		if !slices.ContainsFunc(neighbors, func(item placedZoneRef) bool {
			return item.TileInstanceID == neighborRef.TileInstanceID && item.ZoneID == neighborRef.ZoneID
		}) {
			neighbors = append(neighbors, *neighborRef)
		}
	}

	return neighbors, openEdges, nil
}

func (e *Engine) buildFeature(state carcassonneDTO.GameState, start placedZoneRef) (*feature, error) {
	startZone, err := e.getPlacedZone(state, start)
	if err != nil {
		return nil, err
	}

	result := &feature{
		Type:           startZone.Zone.Type,
		Zones:          make([]placedZoneRef, 0),
		OpenEdges:      0,
		MeeplesByActor: make(map[string]int),
	}

	queue := []placedZoneRef{start}
	visited := make(map[placedZoneRef]struct{})

	for len(queue) > 0 {
		currentRef := queue[0]
		queue = queue[1:]

		if _, ok := visited[currentRef]; ok {
			continue
		}
		visited[currentRef] = struct{}{}

		currentZone, err := e.getPlacedZone(state, currentRef)
		if err != nil {
			return nil, err
		}
		if currentZone.Zone.Type != result.Type {
			return nil, fmt.Errorf("mixed zone types in feature: %s and %s", result.Type, currentZone.Zone.Type)
		}

		result.Zones = append(result.Zones, currentRef)

		for _, meeple := range state.Meeples {
			if meeple.TileInstanceID == currentRef.TileInstanceID && meeple.ZoneID == currentRef.ZoneID {
				result.MeeplesByActor[meeple.ActorID]++
			}
		}

		neighbors, openEdges, err := e.connectedNeighborZones(state, currentZone)
		if err != nil {
			return nil, err
		}
		result.OpenEdges += openEdges

		for _, neighbor := range neighbors {
			if _, ok := visited[neighbor]; !ok {
				queue = append(queue, neighbor)
			}
		}
	}

	return result, nil
}
