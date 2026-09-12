package event

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/webmafia/tumladan/internal/model"
)

const (
	MatchEventsTopic  = "tumladan.match-events.v1"
	MatchFinishedType = "match.finished.v1"
)

type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurredAt"`
	Data       json.RawMessage `json:"data"`
}

type MatchFinished struct {
	MatchID           string                       `json:"matchId"`
	RoomID            string                       `json:"roomId"`
	GameType          string                       `json:"gameType"`
	TerminationReason model.MatchTerminationReason `json:"terminationReason"`
	TerminatedAt      time.Time                    `json:"terminatedAt"`
	Players           []MatchFinishedPlayer        `json:"players"`
}

type MatchFinishedPlayer struct {
	ActorID     string          `json:"actorId"`
	ActorType   model.ActorType `json:"actorType"`
	DisplayName string          `json:"displayName"`
	Seat        int             `json:"seat"`
	Score       int             `json:"score"`
	IsWinner    bool            `json:"isWinner"`
	IsDraw      bool            `json:"isDraw"`
}

type storedResult struct {
	Winners     []string `json:"winners"`
	FinalScores []struct {
		ActorID string `json:"actorId"`
		Score   int    `json:"score"`
	} `json:"finalScores"`
}

func NewMatchFinished(
	match *model.Match,
	players []model.MatchPlayer,
	result *model.JSONB,
	reason model.MatchTerminationReason,
	occurredAt time.Time,
) (Envelope, error) {
	var stored storedResult
	if result != nil {
		if err := json.Unmarshal(*result, &stored); err != nil {
			return Envelope{}, err
		}
	}

	scores := make(map[string]int, len(stored.FinalScores))
	for _, score := range stored.FinalScores {
		scores[score.ActorID] = score.Score
	}
	winners := make(map[string]struct{}, len(stored.Winners))
	for _, actorID := range stored.Winners {
		winners[actorID] = struct{}{}
	}

	payload := MatchFinished{
		MatchID:           match.ID,
		RoomID:            match.RoomID,
		GameType:          match.GameType,
		TerminationReason: reason,
		TerminatedAt:      occurredAt,
		Players:           make([]MatchFinishedPlayer, 0, len(players)),
	}
	for _, player := range players {
		_, winner := winners[player.ActorID]
		payload.Players = append(payload.Players, MatchFinishedPlayer{
			ActorID:     player.ActorID,
			ActorType:   player.ActorType,
			DisplayName: player.DisplayName,
			Seat:        player.Seat,
			Score:       scores[player.ActorID],
			IsWinner:    winner,
			IsDraw:      winner && len(stored.Winners) > 1,
		})
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ID:         uuid.NewString(),
		Type:       MatchFinishedType,
		Version:    1,
		OccurredAt: occurredAt,
		Data:       data,
	}, nil
}
