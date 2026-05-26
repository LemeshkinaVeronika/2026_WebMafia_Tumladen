package service

import (
	"encoding/json"
	"fmt"
	"slices"

	carcassonneAssets "github.com/webmafia/tumladan/internal/game/carcassonne/assets"
	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

type TileCatalog interface {
	Get(tileID string) (carcassonneDTO.TileDefinition, bool)
	Definitions() []carcassonneDTO.TileDefinition
	BaseDeck() []string
	StartTileID() string
}

type AssetTileCatalog struct {
	definitions []carcassonneDTO.TileDefinition
	byID        map[string]carcassonneDTO.TileDefinition
	startTileID string
}

func NewAssetTileCatalog() (*AssetTileCatalog, error) {
	var definitions []carcassonneDTO.TileDefinition
	if err := json.Unmarshal(carcassonneAssets.BaseTilesJSON, &definitions); err != nil {
		return nil, fmt.Errorf("unmarshal tile catalog: %w", err)
	}

	if err := validateTileDefinitions(definitions); err != nil {
		return nil, fmt.Errorf("validate tile catalog: %w", err)
	}

	byID := make(map[string]carcassonneDTO.TileDefinition, len(definitions))
	for _, def := range definitions {
		byID[def.TileID] = def
	}

	return &AssetTileCatalog{
		definitions: definitions,
		byID:        byID,
		startTileID: "start_tile",
	}, nil
}

func (c *AssetTileCatalog) Get(tileID string) (carcassonneDTO.TileDefinition, bool) {
	def, ok := c.byID[tileID]
	return def, ok
}

func (c *AssetTileCatalog) Definitions() []carcassonneDTO.TileDefinition {
	result := make([]carcassonneDTO.TileDefinition, len(c.definitions))
	copy(result, c.definitions)
	return result
}

func (c *AssetTileCatalog) BaseDeck() []string {
	deck := make([]string, 0)
	for _, def := range c.definitions {
		if def.TileID == c.startTileID {
			continue
		}
		for i := 0; i < def.Count; i++ {
			deck = append(deck, def.TileID)
		}
	}
	return deck
}

func (c *AssetTileCatalog) StartTileID() string {
	return c.startTileID
}

func validateTileDefinitions(definitions []carcassonneDTO.TileDefinition) error {
	if len(definitions) == 0 {
		return fmt.Errorf("empty tile catalog")
	}

	allowedSegments := map[carcassonneDTO.ZoneSegment]struct{}{
		carcassonneDTO.SegmentTopLeft: {}, carcassonneDTO.SegmentTopCenter: {}, carcassonneDTO.SegmentTopRight: {},
		carcassonneDTO.SegmentRightTop: {}, carcassonneDTO.SegmentRightCenter: {}, carcassonneDTO.SegmentRightBottom: {},
		carcassonneDTO.SegmentBottomRight: {}, carcassonneDTO.SegmentBottomCenter: {}, carcassonneDTO.SegmentBottomLeft: {},
		carcassonneDTO.SegmentLeftBottom: {}, carcassonneDTO.SegmentLeftCenter: {}, carcassonneDTO.SegmentLeftTop: {},
		carcassonneDTO.SegmentCenter: {},
	}

	seenTileIDs := make(map[string]struct{}, len(definitions))

	for _, def := range definitions {
		if def.TileID == "" {
			return fmt.Errorf("tileId is required")
		}
		if _, exists := seenTileIDs[def.TileID]; exists {
			return fmt.Errorf("duplicate tileId: %s", def.TileID)
		}
		seenTileIDs[def.TileID] = struct{}{}

		if def.ImageKey == "" {
			return fmt.Errorf("imageKey is required for tile %s", def.TileID)
		}
		if def.Count < 0 {
			return fmt.Errorf("count must be >= 0 for tile %s", def.TileID)
		}
		if len(def.Zones) == 0 {
			return fmt.Errorf("zones are required for tile %s", def.TileID)
		}

		seenZoneIDs := make(map[string]struct{}, len(def.Zones))
		usedSegments := make(map[carcassonneDTO.ZoneSegment]string)
		segmentTypes := make(map[carcassonneDTO.ZoneSegment]carcassonneDTO.ZoneType)

		for _, zone := range def.Zones {
			if zone.ZoneID == "" {
				return fmt.Errorf("zoneId is required for tile %s", def.TileID)
			}
			if _, exists := seenZoneIDs[zone.ZoneID]; exists {
				return fmt.Errorf("duplicate zoneId %s in tile %s", zone.ZoneID, def.TileID)
			}
			seenZoneIDs[zone.ZoneID] = struct{}{}

			if len(zone.Segments) == 0 {
				return fmt.Errorf("zone %s in tile %s must have at least one segment", zone.ZoneID, def.TileID)
			}

			seenZoneSegments := make(map[carcassonneDTO.ZoneSegment]struct{}, len(zone.Segments))
			for _, segment := range zone.Segments {
				if _, ok := allowedSegments[segment]; !ok {
					return fmt.Errorf("unknown segment %q in zone %s of tile %s", segment, zone.ZoneID, def.TileID)
				}
				if _, exists := seenZoneSegments[segment]; exists {
					return fmt.Errorf("duplicate segment %q in zone %s of tile %s", segment, zone.ZoneID, def.TileID)
				}
				seenZoneSegments[segment] = struct{}{}

				if ownerZoneID, exists := usedSegments[segment]; exists {
					return fmt.Errorf("segment %q used by both %s and %s in tile %s", segment, ownerZoneID, zone.ZoneID, def.TileID)
				}
				usedSegments[segment] = zone.ZoneID
				segmentTypes[segment] = zone.Type
			}

			if zone.HasPennant && zone.Type != carcassonneDTO.ZoneTypeCity {
				return fmt.Errorf("zone %s in tile %s has pennant but is not city", zone.ZoneID, def.TileID)
			}
			if zone.Type == carcassonneDTO.ZoneTypeField && !zoneHasBoundarySegment(zone) {
				return fmt.Errorf("field zone %s in tile %s must include a boundary segment", zone.ZoneID, def.TileID)
			}
		}

		for _, segment := range boundarySegments() {
			if _, ok := usedSegments[segment]; !ok {
				return fmt.Errorf("boundary segment %q is not used in tile %s", segment, def.TileID)
			}
		}
		if err := validateEdgeSegments(def, segmentTypes); err != nil {
			return err
		}

		if def.TileID == "start_tile" && def.Count != 0 {
			return fmt.Errorf("start_tile must have count 0")
		}
	}

	if _, ok := seenTileIDs["start_tile"]; !ok {
		return fmt.Errorf("start_tile definition is required")
	}

	return nil
}

func zoneHasBoundarySegment(zone carcassonneDTO.ZoneDefinition) bool {
	for _, segment := range zone.Segments {
		if segment != carcassonneDTO.SegmentCenter {
			return true
		}
	}
	return false
}

func boundarySegments() []carcassonneDTO.ZoneSegment {
	return []carcassonneDTO.ZoneSegment{
		carcassonneDTO.SegmentTopLeft,
		carcassonneDTO.SegmentTopCenter,
		carcassonneDTO.SegmentTopRight,
		carcassonneDTO.SegmentRightTop,
		carcassonneDTO.SegmentRightCenter,
		carcassonneDTO.SegmentRightBottom,
		carcassonneDTO.SegmentBottomRight,
		carcassonneDTO.SegmentBottomCenter,
		carcassonneDTO.SegmentBottomLeft,
		carcassonneDTO.SegmentLeftBottom,
		carcassonneDTO.SegmentLeftCenter,
		carcassonneDTO.SegmentLeftTop,
	}
}

func validateEdgeSegments(def carcassonneDTO.TileDefinition, segmentTypes map[carcassonneDTO.ZoneSegment]carcassonneDTO.ZoneType) error {
	edges := []struct {
		edge     carcassonneDTO.EdgeType
		segments []carcassonneDTO.ZoneSegment
	}{
		{
			edge: def.Edges.Top,
			segments: []carcassonneDTO.ZoneSegment{
				carcassonneDTO.SegmentTopLeft,
				carcassonneDTO.SegmentTopCenter,
				carcassonneDTO.SegmentTopRight,
			},
		},
		{
			edge: def.Edges.Right,
			segments: []carcassonneDTO.ZoneSegment{
				carcassonneDTO.SegmentRightTop,
				carcassonneDTO.SegmentRightCenter,
				carcassonneDTO.SegmentRightBottom,
			},
		},
		{
			edge: def.Edges.Bottom,
			segments: []carcassonneDTO.ZoneSegment{
				carcassonneDTO.SegmentBottomRight,
				carcassonneDTO.SegmentBottomCenter,
				carcassonneDTO.SegmentBottomLeft,
			},
		},
		{
			edge: def.Edges.Left,
			segments: []carcassonneDTO.ZoneSegment{
				carcassonneDTO.SegmentLeftBottom,
				carcassonneDTO.SegmentLeftCenter,
				carcassonneDTO.SegmentLeftTop,
			},
		},
	}

	for _, edge := range edges {
		if edge.edge == carcassonneDTO.EdgeTypeRoad {
			center := edge.segments[1]
			if segmentTypes[center] != carcassonneDTO.ZoneTypeRoad {
				return fmt.Errorf("road edge center segment %q in tile %s must be road", center, def.TileID)
			}
			continue
		}

		expected := carcassonneDTO.ZoneType(edge.edge)
		for _, segment := range edge.segments {
			if segmentTypes[segment] != expected {
				return fmt.Errorf("%s edge segment %q in tile %s must be %s", edge.edge, segment, def.TileID, expected)
			}
		}
	}

	return nil
}

func sortTileDefinitions(definitions []carcassonneDTO.TileDefinition) {
	slices.SortFunc(definitions, func(a, b carcassonneDTO.TileDefinition) int {
		switch {
		case a.TileID < b.TileID:
			return -1
		case a.TileID > b.TileID:
			return 1
		default:
			return 0
		}
	})
}
