package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/webmafia/tumladan/internal/event"
)

type repositoryStub struct {
	group   string
	eventID string
	match   event.MatchFinished
	calls   int
}

func (r *repositoryStub) ApplyMatchFinished(_ context.Context, group, eventID string, match event.MatchFinished) (bool, error) {
	r.group, r.eventID, r.match = group, eventID, match
	r.calls++
	return true, nil
}

func TestConsumerHandlesMatchFinished(t *testing.T) {
	data, _ := json.Marshal(event.MatchFinished{MatchID: "match-1", GameType: "carcassonne"})
	value, _ := json.Marshal(event.Envelope{
		ID:         "50e71ca7-f4ad-4e88-a7bc-48e08e740c4e",
		Type:       event.MatchFinishedType,
		Version:    1,
		OccurredAt: time.Now(),
		Data:       data,
	})
	repo := &repositoryStub{}
	applied, err := NewConsumer(repo, "stats-v1").Handle(context.Background(), value)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if !applied || repo.calls != 1 || repo.group != "stats-v1" || repo.match.MatchID != "match-1" {
		t.Fatalf("unexpected repository call: %+v", repo)
	}
}

func TestConsumerRejectsUnknownEventVersion(t *testing.T) {
	value, _ := json.Marshal(event.Envelope{ID: "event-1", Type: event.MatchFinishedType, Version: 2})
	if _, err := NewConsumer(&repositoryStub{}, "stats-v1").Handle(context.Background(), value); err == nil {
		t.Fatal("Handle() error = nil, want unsupported version error")
	}
}
