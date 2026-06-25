package service

import (
	"slices"

	carcassonneDTO "github.com/webmafia/tumladan/internal/game/carcassonne/dto"
)

type disjointSet struct {
	parent map[placedFieldSegmentRef]placedFieldSegmentRef
	size   map[placedFieldSegmentRef]int
}

func newDisjointSet() *disjointSet {
	return &disjointSet{
		parent: make(map[placedFieldSegmentRef]placedFieldSegmentRef),
		size:   make(map[placedFieldSegmentRef]int),
	}
}

func (d *disjointSet) add(ref placedFieldSegmentRef) {
	if _, ok := d.parent[ref]; ok {
		return
	}
	d.parent[ref] = ref
	d.size[ref] = 1
}

func (d *disjointSet) find(ref placedFieldSegmentRef) (placedFieldSegmentRef, bool) {
	parent, ok := d.parent[ref]
	if !ok {
		return placedFieldSegmentRef{}, false
	}
	if parent == ref {
		return ref, true
	}
	root, ok := d.find(parent)
	if !ok {
		return placedFieldSegmentRef{}, false
	}
	d.parent[ref] = root
	return root, true
}

func (d *disjointSet) union(a, b placedFieldSegmentRef) {
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

type placedFieldSegmentRef struct {
	TileInstanceID string
	ZoneID         string
	Segment        carcassonneDTO.ZoneSegment
}

type fieldSegment struct {
	Ref     placedFieldSegmentRef
	Tile    carcassonneDTO.PlacedTile
	Zone    carcassonneDTO.ZoneDefinition
	Segment carcassonneDTO.ZoneSegment
}

type fieldGraph struct {
	dsu                   *disjointSet
	segmentsByRoot        map[placedFieldSegmentRef][]placedFieldSegmentRef
	meeplesByRoot         map[placedFieldSegmentRef]map[string]int
	fieldSegmentByRef     map[placedFieldSegmentRef]fieldSegment
	segmentsByZoneRef     map[placedZoneRef][]placedFieldSegmentRef
	completedCityFeatures map[placedZoneRef]struct{}
}

func (e *Engine) buildFieldGraph(state carcassonneDTO.GameState) (*fieldGraph, error) {
	graph := &fieldGraph{
		dsu:                   newDisjointSet(),
		segmentsByRoot:        make(map[placedFieldSegmentRef][]placedFieldSegmentRef),
		meeplesByRoot:         make(map[placedFieldSegmentRef]map[string]int),
		fieldSegmentByRef:     make(map[placedFieldSegmentRef]fieldSegment),
		segmentsByZoneRef:     make(map[placedZoneRef][]placedFieldSegmentRef),
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

			zoneRef := placedZoneRef{
				TileInstanceID: tile.InstanceID,
				ZoneID:         zone.ZoneID,
			}

			segments := rotateSegments(zone.Segments, tile.Rotation)
			for _, segment := range segments {
				ref := placedFieldSegmentRef{
					TileInstanceID: tile.InstanceID,
					ZoneID:         zone.ZoneID,
					Segment:        segment,
				}
				graph.dsu.add(ref)
				graph.fieldSegmentByRef[ref] = fieldSegment{
					Ref:     ref,
					Tile:    tile,
					Zone:    zone,
					Segment: segment,
				}
				graph.segmentsByZoneRef[zoneRef] = append(graph.segmentsByZoneRef[zoneRef], ref)
			}

			for i := 1; i < len(segments); i++ {
				graph.dsu.union(
					placedFieldSegmentRef{TileInstanceID: tile.InstanceID, ZoneID: zone.ZoneID, Segment: segments[0]},
					placedFieldSegmentRef{TileInstanceID: tile.InstanceID, ZoneID: zone.ZoneID, Segment: segments[i]},
				)
			}
		}
	}

	for _, segment := range graph.fieldSegmentByRef {
		match, ok := borderNeighbor(segment.Segment)
		if !ok {
			continue
		}

		neighborTile := tileAt(state.Board, segment.Tile.X+match.DX, segment.Tile.Y+match.DY)
		if neighborTile == nil {
			continue
		}

		neighborRef, err := e.findNeighborFieldSegmentBySegment(
			state,
			neighborTile.X,
			neighborTile.Y,
			match.NeighborSegment,
		)
		if err != nil {
			return nil, err
		}
		if neighborRef == nil {
			continue
		}

		graph.dsu.union(segment.Ref, *neighborRef)
	}

	for ref := range graph.fieldSegmentByRef {
		root, ok := graph.dsu.find(ref)
		if !ok {
			continue
		}
		graph.segmentsByRoot[root] = append(graph.segmentsByRoot[root], ref)
	}

	for _, meeple := range state.Meeples {
		zoneRef := placedZoneRef{TileInstanceID: meeple.TileInstanceID, ZoneID: meeple.ZoneID}
		countedRoots := make(map[placedFieldSegmentRef]struct{})
		for _, segmentRef := range graph.segmentsByZoneRef[zoneRef] {
			root, ok := graph.dsu.find(segmentRef)
			if !ok {
				continue
			}
			if _, ok := countedRoots[root]; ok {
				continue
			}
			countedRoots[root] = struct{}{}
			if graph.meeplesByRoot[root] == nil {
				graph.meeplesByRoot[root] = make(map[string]int)
			}
			graph.meeplesByRoot[root][meeple.ActorID] += meepleWeight(meeple)
		}
	}

	if err := e.collectCompletedCityFeatures(state, graph.completedCityFeatures); err != nil {
		return nil, err
	}

	return graph, nil
}

func (g *fieldGraph) rootsForZone(ref placedZoneRef) []placedFieldSegmentRef {
	if g == nil {
		return nil
	}

	roots := make([]placedFieldSegmentRef, 0)
	for _, segmentRef := range g.segmentsByZoneRef[ref] {
		root, ok := g.dsu.find(segmentRef)
		if !ok {
			continue
		}
		if slices.Contains(roots, root) {
			continue
		}
		roots = append(roots, root)
	}

	return roots
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

func (e *Engine) fieldCompletedCityCount(state carcassonneDTO.GameState, graph *fieldGraph, root placedFieldSegmentRef) (int, error) {
	if graph == nil {
		return 0, nil
	}

	seen := make(map[placedZoneRef]struct{})
	for _, fieldRef := range graph.segmentsByRoot[root] {
		fieldSegment := graph.fieldSegmentByRef[fieldRef]
		cityRefs := e.adjacentCityZoneRefs(state, fieldSegment)
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

func (e *Engine) adjacentCityZoneRefs(state carcassonneDTO.GameState, fieldSegment fieldSegment) []placedZoneRef {
	def, ok := e.catalog.Get(fieldSegment.Tile.TileID)
	if !ok {
		return nil
	}

	result := make([]placedZoneRef, 0)
	for _, zone := range def.Zones {
		if zone.Type != carcassonneDTO.ZoneTypeCity {
			continue
		}

		citySegments := rotateSegments(zone.Segments, fieldSegment.Tile.Rotation)
		if !segmentsTouch([]carcassonneDTO.ZoneSegment{fieldSegment.Segment}, citySegments) {
			continue
		}

		result = append(result, placedZoneRef{
			TileInstanceID: fieldSegment.Tile.InstanceID,
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
	for _, adjacent := range adjacentSegments(a) {
		if adjacent == b {
			return true
		}
	}
	return false
}

func adjacentSegments(segment carcassonneDTO.ZoneSegment) []carcassonneDTO.ZoneSegment {
	switch segment {
	case carcassonneDTO.SegmentTopLeft:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentTopCenter, carcassonneDTO.SegmentLeftTop}
	case carcassonneDTO.SegmentTopCenter:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentTopLeft, carcassonneDTO.SegmentTopRight}
	case carcassonneDTO.SegmentTopRight:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentTopCenter, carcassonneDTO.SegmentRightTop}
	case carcassonneDTO.SegmentRightTop:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentTopRight, carcassonneDTO.SegmentRightCenter}
	case carcassonneDTO.SegmentRightCenter:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentRightTop, carcassonneDTO.SegmentRightBottom}
	case carcassonneDTO.SegmentRightBottom:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentRightCenter, carcassonneDTO.SegmentBottomRight}
	case carcassonneDTO.SegmentBottomRight:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentRightBottom, carcassonneDTO.SegmentBottomCenter}
	case carcassonneDTO.SegmentBottomCenter:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentBottomRight, carcassonneDTO.SegmentBottomLeft}
	case carcassonneDTO.SegmentBottomLeft:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentBottomCenter, carcassonneDTO.SegmentLeftBottom}
	case carcassonneDTO.SegmentLeftBottom:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentBottomLeft, carcassonneDTO.SegmentLeftCenter}
	case carcassonneDTO.SegmentLeftCenter:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentLeftBottom, carcassonneDTO.SegmentLeftTop}
	case carcassonneDTO.SegmentLeftTop:
		return []carcassonneDTO.ZoneSegment{carcassonneDTO.SegmentLeftCenter, carcassonneDTO.SegmentTopLeft}
	default:
		return nil
	}
}
