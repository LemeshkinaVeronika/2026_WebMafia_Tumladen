package outbox

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID        string
	Topic     string
	Key       string
	Type      string
	Payload   json.RawMessage
	Attempts  int
	CreatedAt time.Time
}
