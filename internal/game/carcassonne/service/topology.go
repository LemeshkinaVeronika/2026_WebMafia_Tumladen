package service

import (
	"fmt"
	"slices"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

type placedZoneRef struct {
	TileInstanceID string
	ZoneID         string
}

type placedZone struct {
	Ref      placedZoneRef
	Tile     carcassonneDTO.PlacedTile
	Zone     carcassonneDTO.ZoneDefinition
	Segments []carcassonneDTO.ZoneSegment
}

type borderMatch struct {
	DX              int
	DY              int
	NeighborSegment carcassonneDTO.ZoneSegment
}

func rotateSegment(segment carcassonneDTO.ZoneSegment, rotation int) carcassonneDTO.ZoneSegment {
	switch normalizeRotation(rotation) {
	case 0:
		return segment
	case 90:
		switch segment {
		case carcassonneDTO.SegmentTopLeft:
			return carcassonneDTO.SegmentRightTop
		case carcassonneDTO.SegmentTopCenter:
			return carcassonneDTO.SegmentRightCenter
		case carcassonneDTO.SegmentTopRight:
			return carcassonneDTO.SegmentRightBottom

		case carcassonneDTO.SegmentRightTop:
			return carcassonneDTO.SegmentBottomRight
		case carcassonneDTO.SegmentRightCenter:
			return carcassonneDTO.SegmentBottomCenter
		case carcassonneDTO.SegmentRightBottom:
			return carcassonneDTO.SegmentBottomLeft

		case carcassonneDTO.SegmentBottomRight:
			return carcassonneDTO.SegmentLeftBottom
		case carcassonneDTO.SegmentBottomCenter:
			return carcassonneDTO.SegmentLeftCenter
		case carcassonneDTO.SegmentBottomLeft:
			return carcassonneDTO.SegmentLeftTop

		case carcassonneDTO.SegmentLeftBottom:
			return carcassonneDTO.SegmentTopLeft
		case carcassonneDTO.SegmentLeftCenter:
			return carcassonneDTO.SegmentTopCenter
		case carcassonneDTO.SegmentLeftTop:
			return carcassonneDTO.SegmentTopRight

		case carcassonneDTO.SegmentCenter:
			return carcassonneDTO.SegmentCenter
		}
	case 180:
		return rotateSegment(rotateSegment(segment, 90), 90)
	case 270:
		return rotateSegment(rotateSegment(segment, 180), 90)
	}

	return segment
}

func rotateSegments(segments []carcassonneDTO.ZoneSegment, rotation int) []carcassonneDTO.ZoneSegment {
	result := make([]carcassonneDTO.ZoneSegment, 0, len(segments))
	for _, segment := range segments {
		result = append(result, rotateSegment(segment, rotation))
	}
	return result
}

func findZone(def carcassonneDTO.TileDefinition, zoneID string) (*carcassonneDTO.ZoneDefinition, bool) {
	for i := range def.Zones {
		if def.Zones[i].ZoneID == zoneID {
			return &def.Zones[i], true
		}
	}
	return nil, false
}

func borderNeighbor(segment carcassonneDTO.ZoneSegment) (borderMatch, bool) {
	switch segment {
	case carcassonneDTO.SegmentTopLeft:
		return borderMatch{
			DX:              0,
			DY:              -1,
			NeighborSegment: carcassonneDTO.SegmentBottomLeft,
		}, true
	case carcassonneDTO.SegmentTopCenter:
		return borderMatch{
			DX:              0,
			DY:              -1,
			NeighborSegment: carcassonneDTO.SegmentBottomCenter,
		}, true
	case carcassonneDTO.SegmentTopRight:
		return borderMatch{
			DX:              0,
			DY:              -1,
			NeighborSegment: carcassonneDTO.SegmentBottomRight,
		}, true

	case carcassonneDTO.SegmentRightTop:
		return borderMatch{
			DX:              1,
			DY:              0,
			NeighborSegment: carcassonneDTO.SegmentLeftTop,
		}, true
	case carcassonneDTO.SegmentRightCenter:
		return borderMatch{
			DX:              1,
			DY:              0,
			NeighborSegment: carcassonneDTO.SegmentLeftCenter,
		}, true
	case carcassonneDTO.SegmentRightBottom:
		return borderMatch{
			DX:              1,
			DY:              0,
			NeighborSegment: carcassonneDTO.SegmentLeftBottom,
		}, true

	case carcassonneDTO.SegmentBottomRight:
		return borderMatch{
			DX:              0,
			DY:              1,
			NeighborSegment: carcassonneDTO.SegmentTopRight,
		}, true
	case carcassonneDTO.SegmentBottomCenter:
		return borderMatch{
			DX:              0,
			DY:              1,
			NeighborSegment: carcassonneDTO.SegmentTopCenter,
		}, true
	case carcassonneDTO.SegmentBottomLeft:
		return borderMatch{
			DX:              0,
			DY:              1,
			NeighborSegment: carcassonneDTO.SegmentTopLeft,
		}, true

	case carcassonneDTO.SegmentLeftBottom:
		return borderMatch{
			DX:              -1,
			DY:              0,
			NeighborSegment: carcassonneDTO.SegmentRightBottom,
		}, true
	case carcassonneDTO.SegmentLeftCenter:
		return borderMatch{
			DX:              -1,
			DY:              0,
			NeighborSegment: carcassonneDTO.SegmentRightCenter,
		}, true
	case carcassonneDTO.SegmentLeftTop:
		return borderMatch{
			DX:              -1,
			DY:              0,
			NeighborSegment: carcassonneDTO.SegmentRightTop,
		}, true

	case carcassonneDTO.SegmentCenter:
		return borderMatch{}, false
	default:
		return borderMatch{}, false
	}
}

func (e *Engine) getPlacedZone(state carcassonneDTO.GameState, ref placedZoneRef) (*placedZone, error) {
	var tile *carcassonneDTO.PlacedTile
	for i := range state.Board {
		if state.Board[i].InstanceID == ref.TileInstanceID {
			tile = &state.Board[i]
			break
		}
	}
	if tile == nil {
		return nil, fmt.Errorf("placed tile not found: %s", ref.TileInstanceID)
	}

	def, ok := e.catalog.Get(tile.TileID)
	if !ok {
		return nil, fmt.Errorf("tile definition not found: %s", tile.TileID)
	}

	zone, ok := findZone(def, ref.ZoneID)
	if !ok {
		return nil, fmt.Errorf("zone definition not found: tile=%s zone=%s", tile.TileID, ref.ZoneID)
	}

	return &placedZone{
		Ref:  ref,
		Tile: *tile,
		Zone: *zone,
		Segments: rotateSegments(
			zone.Segments,
			tile.Rotation,
		),
	}, nil
}

func containsSegment(segments []carcassonneDTO.ZoneSegment, target carcassonneDTO.ZoneSegment) bool {
	return slices.Contains(segments, target)
}

func (e *Engine) findNeighborZoneBySegment(
	state carcassonneDTO.GameState,
	x, y int,
	zoneType carcassonneDTO.ZoneType,
	segment carcassonneDTO.ZoneSegment,
) (*placedZoneRef, error) {
	neighborTile := tileAt(state.Board, x, y)
	if neighborTile == nil {
		return nil, nil
	}

	def, ok := e.catalog.Get(neighborTile.TileID)
	if !ok {
		return nil, fmt.Errorf("tile definition not found: %s", neighborTile.TileID)
	}

	for _, zone := range def.Zones {
		if zone.Type != zoneType {
			continue
		}

		rotated := rotateSegments(zone.Segments, neighborTile.Rotation)
		if containsSegment(rotated, segment) {
			return &placedZoneRef{
				TileInstanceID: neighborTile.InstanceID,
				ZoneID:         zone.ZoneID,
			}, nil
		}
	}

	return nil, nil
}
