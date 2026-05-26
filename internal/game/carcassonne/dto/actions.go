package dto

type PlaceTilePayload struct {
	RoomID   string `json:"roomId"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Rotation int    `json:"rotation"`
}

type PlaceMeeplePayload struct {
	RoomID  string      `json:"roomId"`
	ZoneID  string      `json:"zoneId"`
	Segment ZoneSegment `json:"segment,omitempty"`
}

type SkipMeeplePayload struct {
	RoomID string `json:"roomId"`
}

type Action string

const (
	ActionPlaceTile   Action = "place_tile"
	ActionPlaceMeeple Action = "place_meeple"
	ActionSkipMeeple  Action = "skip_meeple"
)
