package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/webmafia/tumladan/internal/event"
)

type Repository interface {
	ApplyMatchFinished(ctx context.Context, consumerGroup, eventID string, match event.MatchFinished) (bool, error)
}

type Consumer struct {
	repository    Repository
	consumerGroup string
}

func NewConsumer(repository Repository, consumerGroup string) *Consumer {
	return &Consumer{repository: repository, consumerGroup: consumerGroup}
}

func (c *Consumer) Handle(ctx context.Context, value []byte) (bool, error) {
	var envelope event.Envelope
	if err := json.Unmarshal(value, &envelope); err != nil {
		return false, fmt.Errorf("decode event envelope: %w", err)
	}
	if envelope.ID == "" {
		return false, fmt.Errorf("event id is required")
	}
	if _, err := uuid.Parse(envelope.ID); err != nil {
		return false, fmt.Errorf("invalid event id: %w", err)
	}
	if envelope.Type != event.MatchFinishedType || envelope.Version != 1 {
		return false, fmt.Errorf("unsupported event %q version %d", envelope.Type, envelope.Version)
	}

	var match event.MatchFinished
	if err := json.Unmarshal(envelope.Data, &match); err != nil {
		return false, fmt.Errorf("decode match event: %w", err)
	}
	return c.repository.ApplyMatchFinished(ctx, c.consumerGroup, envelope.ID, match)
}
