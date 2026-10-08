package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMatchActivityResponseMarshalsPayloadAsJSON(t *testing.T) {
	raw, err := json.Marshal(MatchResponse{
		RecentActions: []MatchActivityResponse{
			{
				ID:      "activity-1",
				Type:    "tile_placed",
				Payload: json.RawMessage(`{"tileId":"city_cap"}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(raw), `"payload":{"tileId":"city_cap"}`) {
		t.Fatalf("marshaled response = %s, want raw JSON activity payload", raw)
	}
}
