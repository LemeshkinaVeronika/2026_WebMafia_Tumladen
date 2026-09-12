package service

import (
	"context"
	"math"
	"time"

	"github.com/webmafia/tumladan/internal/outbox"
	"github.com/webmafia/tumladan/pkg/logger"
)

type Repository interface {
	ClaimBatch(ctx context.Context, limit int, lease time.Duration) ([]outbox.Event, error)
	MarkPublished(ctx context.Context, eventID string) error
	MarkFailed(ctx context.Context, eventID, message string, retryAfter time.Duration) error
}

type Producer interface {
	Publish(ctx context.Context, event outbox.Event) error
}

type Publisher struct {
	repository   Repository
	producer     Producer
	logger       logger.Logger
	pollInterval time.Duration
	batchSize    int
	lease        time.Duration
	maxAttempts  int
	dlqTopic     string
}

func NewPublisher(repository Repository, producer Producer, logger logger.Logger, pollInterval time.Duration, batchSize, maxAttempts int, dlqTopic string) *Publisher {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	return &Publisher{
		repository:   repository,
		producer:     producer,
		logger:       logger,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		lease:        30 * time.Second,
		maxAttempts:  maxAttempts,
		dlqTopic:     dlqTopic,
	}
}

func (p *Publisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		if err := p.publishBatch(ctx); err != nil && ctx.Err() == nil {
			p.logger.With("error", err).Warnf("outbox batch failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Publisher) publishBatch(ctx context.Context) error {
	events, err := p.repository.ClaimBatch(ctx, p.batchSize, p.lease)
	if err != nil {
		return err
	}
	for _, event := range events {
		publishEvent := event
		if event.Attempts > p.maxAttempts {
			publishEvent.Topic = p.dlqTopic
		}
		if err := p.producer.Publish(ctx, publishEvent); err != nil {
			retryAfter := retryDelay(event.Attempts)
			if markErr := p.repository.MarkFailed(ctx, event.ID, err.Error(), retryAfter); markErr != nil {
				return markErr
			}
			continue
		}
		if err := p.repository.MarkPublished(ctx, event.ID); err != nil {
			return err
		}
	}
	return nil
}

func retryDelay(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	seconds := math.Pow(2, float64(min(attempt, 8)))
	return min(time.Duration(seconds)*time.Second, 5*time.Minute)
}
