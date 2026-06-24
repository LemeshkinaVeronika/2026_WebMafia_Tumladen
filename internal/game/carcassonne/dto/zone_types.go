package dto

type ZoneType string

const (
	ZoneTypeCity      ZoneType = "city"
	ZoneTypeRoad      ZoneType = "road"
	ZoneTypeMonastery ZoneType = "monastery"
	ZoneTypeField     ZoneType = "field"
	ZonaTypeCathedral ZoneType = "cathedral"
)

type EdgeType string

const (
	EdgeTypeCity  EdgeType = "city"
	EdgeTypeRoad  EdgeType = "road"
	EdgeTypeField EdgeType = "field"
)

type ZoneSegment string

const (
	SegmentTopLeft   ZoneSegment = "top_left"
	SegmentTopCenter ZoneSegment = "top_center"
	SegmentTopRight  ZoneSegment = "top_right"

	SegmentRightTop    ZoneSegment = "right_top"
	SegmentRightCenter ZoneSegment = "right_center"
	SegmentRightBottom ZoneSegment = "right_bottom"

	SegmentBottomRight  ZoneSegment = "bottom_right"
	SegmentBottomCenter ZoneSegment = "bottom_center"
	SegmentBottomLeft   ZoneSegment = "bottom_left"

	SegmentLeftBottom ZoneSegment = "left_bottom"
	SegmentLeftCenter ZoneSegment = "left_center"
	SegmentLeftTop    ZoneSegment = "left_top"

	SegmentCenter ZoneSegment = "center"
)

type TileDefinition struct {
	TileID   string           `json:"tileId"`
	ImageKey string           `json:"imageKey"`
	Count    int              `json:"count"`
	Edges    TileEdges        `json:"edges"`
	Zones    []ZoneDefinition `json:"zones"`
}

type TileCatalogItem struct {
	TileID     string           `json:"tileId"`
	ImageURL   string           `json:"imageUrl"`
	Edges      TileEdges        `json:"edges"`
	Zones      []ZoneDefinition `json:"zones"`
	Segments   []ZoneSegment    `json:"segments"`
	HasPennant bool             `json:"hasPennant"`
	Count      int              `json:"count"`
}

type TileEdges struct {
	Top    EdgeType `json:"top"`
	Right  EdgeType `json:"right"`
	Bottom EdgeType `json:"bottom"`
	Left   EdgeType `json:"left"`
}

type ZoneDefinition struct {
	ZoneID     string        `json:"zoneId"`
	Type       ZoneType      `json:"type"`
	Segments   []ZoneSegment `json:"segments,omitempty"`
	HasPennant bool          `json:"hasPennant,omitempty"`
	HasInn     bool          `json:"hasInn,omitempty"`
}
