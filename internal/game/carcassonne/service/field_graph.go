package service

import (
	"slices"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

type disjointSet struct {
	parent map[placedZoneRef]placedZoneRef
	size   map[placedZoneRef]int
}

func newDisjointSet() *disjointSet {
	return &disjointSet{
		parent: make(map[placedZoneRef]placedZoneRef),
		size:   make(map[placedZoneRef]int),
	}
}

func (d *disjointSet) add(ref placedZoneRef) {
	if _, ok := d.parent[ref]; ok {
		return
	}
	d.parent[ref] = ref
	d.size[ref] = 1
}

func (d *disjointSet) find(ref placedZoneRef) (placedZoneRef, bool) {
	parent, ok := d.parent[ref]
	if !ok {
		return placedZoneRef{}, false
	}
	if parent == ref {
		return ref, true
	}
	root, ok := d.find(parent)
	if !ok {
		return placedZoneRef{}, false
	}
	d.parent[ref] = root
	return root, true
}

func (d *disjointSet) union(a, b placedZoneRef) {
	d.add(a)
	d.add(b)

	rootA, _ := d.find(a)
	rootB, _ := d.find(b)
	if rootA == rootB {
		return
	}

	if d.size[rootA] < d.size[rootB] {
		rootA, rootB = rootB, rootA
	}

	d.parent[rootB] = rootA
	d.size[rootA] += d.size[rootB]
	delete(d.size, rootB)
}

type fieldGraph struct {
	dsu                   *disjointSet
	zonesByRoot           map[placedZoneRef][]placedZoneRef
	meeplesByRoot         map[placedZoneRef]map[string]int
	fieldZoneByRef        map[placedZoneRef]placedZone
	completedCityFeatures map[placedZoneRef]struct{}
}

func (e *Engine) buildFieldGraph(state carcassonneDTO.GameState) (*fieldGraph, error) {
	graph := &fieldGraph{
		dsu:                   newDisjointSet(),
		zonesByRoot:           make(map[placedZoneRef][]placedZoneRef),
		meeplesByRoot:         make(map[placedZoneRef]map[string]int),
		fieldZoneByRef:        make(map[placedZoneRef]placedZone),
		completedCityFeatures: make(map[placedZoneRef]struct{}),
	}

	for _, tile := range state.Board {
		def, ok := e.catalog.Get(tile.TileID)
		if !ok {
			continue
		}

		for _, zone := range def.Zones {
			if zone.Type != carcassonneDTO.ZoneTypeField {
				continue
			}

			ref := placedZoneRef{
				TileInstanceID: tile.InstanceID,
				ZoneID:         zone.ZoneID,
			}
			graph.dsu.add(ref)
			graph.fieldZoneByRef[ref] = placedZone{
				Ref:      ref,
				Tile:     tile,
				Zone:     zone,
				Segments: rotateSegments(zone.Segments, tile.Rotation),
			}
		}
	}

	for _, zone := range graph.fieldZoneByRef {
		for _, segment := range zone.Segments {
			match, ok := borderNeighbor(segment)
			if !ok {
				continue
			}

			neighborTile := tileAt(state.Board, zone.Tile.X+match.DX, zone.Tile.Y+match.DY)
			if neighborTile == nil {
				continue
			}

			neighborRef, err := e.findNeighborZoneBySegment(
				state,
				neighborTile.X,
				neighborTile.Y,
				carcassonneDTO.ZoneTypeField,
				match.NeighborSegment,
			)
			if err != nil {
				return nil, err
			}
			if neighborRef == nil {
				continue
			}

			graph.dsu.union(zone.Ref, *neighborRef)
		}
	}

	for ref := range graph.fieldZoneByRef {
		root, ok := graph.dsu.find(ref)
		if !ok {
			continue
		}
		graph.zonesByRoot[root] = append(graph.zonesByRoot[root], ref)
	}

	for _, meeple := range state.Meeples {
		ref := placedZoneRef{TileInstanceID: meeple.TileInstanceID, ZoneID: meeple.ZoneID}
		root, ok := graph.dsu.find(ref)
		if !ok {
			continue
		}
		if graph.meeplesByRoot[root] == nil {
			graph.meeplesByRoot[root] = make(map[string]int)
		}
		graph.meeplesByRoot[root][meeple.ActorID]++
	}

	if err := e.collectCompletedCityFeatures(state, graph.completedCityFeatures); err != nil {
		return nil, err
	}

	return graph, nil
}

func (e *Engine) collectCompletedCityFeatures(state carcassonneDTO.GameState, completed map[placedZoneRef]struct{}) error {
	visited := make(map[placedZoneRef]struct{})

	for _, tile := range state.Board {
		def, ok := e.catalog.Get(tile.TileID)
		if !ok {
			continue
		}

		for _, zone := range def.Zones {
			if zone.Type != carcassonneDTO.ZoneTypeCity {
				continue
			}

			start := placedZoneRef{TileInstanceID: tile.InstanceID, ZoneID: zone.ZoneID}
			if _, ok := visited[start]; ok {
				continue
			}

			feature, err := e.buildFeature(state, start)
			if err != nil {
				return err
			}
			for _, ref := range feature.Zones {
				visited[ref] = struct{}{}
			}
			if feature.OpenEdges == 0 {
				completed[canonicalFeatureRef(feature)] = struct{}{}
			}
		}
	}

	return nil
}

func canonicalFeatureRef(feature *feature) placedZoneRef {
	if feature == nil || len(feature.Zones) == 0 {
		return placedZoneRef{}
	}

	zones := make([]placedZoneRef, len(feature.Zones))
	copy(zones, feature.Zones)
	slices.SortFunc(zones, compareZoneRefs)
	return zones[0]
}

func compareZoneRefs(a, b placedZoneRef) int {
	if a.TileInstanceID != b.TileInstanceID {
		if a.TileInstanceID < b.TileInstanceID {
			return -1
		}
		return 1
	}
	if a.ZoneID < b.ZoneID {
		return -1
	}
	if a.ZoneID > b.ZoneID {
		return 1
	}
	return 0
}

func (e *Engine) fieldCompletedCityCount(state carcassonneDTO.GameState, graph *fieldGraph, root placedZoneRef) (int, error) {
	if graph == nil {
		return 0, nil
	}

	seen := make(map[placedZoneRef]struct{})
	for _, fieldRef := range graph.zonesByRoot[root] {
		fieldZone := graph.fieldZoneByRef[fieldRef]
		cityRefs := e.adjacentCityZoneRefs(state, fieldZone)
		for _, cityRef := range cityRefs {
			feature, err := e.buildFeature(state, cityRef)
			if err != nil {
				return 0, err
			}
			canonical := canonicalFeatureRef(feature)
			if _, ok := graph.completedCityFeatures[canonical]; ok {
				seen[canonical] = struct{}{}
			}
		}
	}

	return len(seen), nil
}

func (e *Engine) adjacentCityZoneRefs(state carcassonneDTO.GameState, fieldZone placedZone) []placedZoneRef {
	def, ok := e.catalog.Get(fieldZone.Tile.TileID)
	if !ok {
		return nil
	}

	result := make([]placedZoneRef, 0)
	for _, zone := range def.Zones {
		if zone.Type != carcassonneDTO.ZoneTypeCity {
			continue
		}

		citySegments := rotateSegments(zone.Segments, fieldZone.Tile.Rotation)
		if !segmentsTouch(fieldZone.Segments, citySegments) {
			continue
		}

		result = append(result, placedZoneRef{
			TileInstanceID: fieldZone.Tile.InstanceID,
			ZoneID:         zone.ZoneID,
		})
	}

	return result
}

func segmentsTouch(a, b []carcassonneDTO.ZoneSegment) bool {
	for _, left := range a {
		for _, right := range b {
			if segmentsAdjacent(left, right) {
				return true
			}
		}
	}
	return false
}

func segmentsAdjacent(a, b carcassonneDTO.ZoneSegment) bool {
	// Approximation for field-city adjacency on the coarse segment grid.
	ax, ay, okA := segmentCoordinate(a)
	bx, by, okB := segmentCoordinate(b)
	if !okA || !okB {
		return false
	}

	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - by
	if dy < 0 {
		dy = -dy
	}

	return dx+dy == 1
}

func segmentCoordinate(segment carcassonneDTO.ZoneSegment) (int, int, bool) {
	switch segment {
	case carcassonneDTO.SegmentTopLeft:
		return 0, 0, true
	case carcassonneDTO.SegmentTopCenter:
		return 1, 0, true
	case carcassonneDTO.SegmentTopRight:
		return 2, 0, true
	case carcassonneDTO.SegmentRightTop:
		return 3, 0, true
	case carcassonneDTO.SegmentRightCenter:
		return 3, 1, true
	case carcassonneDTO.SegmentRightBottom:
		return 3, 2, true
	case carcassonneDTO.SegmentBottomRight:
		return 2, 3, true
	case carcassonneDTO.SegmentBottomCenter:
		return 1, 3, true
	case carcassonneDTO.SegmentBottomLeft:
		return 0, 3, true
	case carcassonneDTO.SegmentLeftBottom:
		return -1, 2, true
	case carcassonneDTO.SegmentLeftCenter:
		return -1, 1, true
	case carcassonneDTO.SegmentLeftTop:
		return -1, 0, true
	case carcassonneDTO.SegmentCenter:
		return 1, 1, true
	default:
		return 0, 0, false
	}
}
