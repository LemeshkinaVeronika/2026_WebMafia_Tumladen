package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/webmafia/tumladan/internal/outbox"
	"github.com/webmafia/tumladan/pkg/logger"
)

type outboxRepositoryStub struct {
	events       []outbox.Event
	publishedIDs []string
	failedIDs    []string
}

func (r *outboxRepositoryStub) ClaimBatch(context.Context, int, time.Duration) ([]outbox.Event, error) {
	return r.events, nil
}
func (r *outboxRepositoryStub) MarkPublished(_ context.Context, id string) error {
	r.publishedIDs = append(r.publishedIDs, id)
	return nil
}
func (r *outboxRepositoryStub) MarkFailed(_ context.Context, id, _ string, _ time.Duration) error {
	r.failedIDs = append(r.failedIDs, id)
	return nil
}

type producerStub struct {
	err    error
	events []outbox.Event
}

func (p *producerStub) Publish(_ context.Context, event outbox.Event) error {
	p.events = append(p.events, event)
	return p.err
}

type loggerStub struct{}

func (loggerStub) Debugf(string, ...interface{})       {}
func (loggerStub) Infof(string, ...interface{})        {}
func (loggerStub) Warnf(string, ...interface{})        {}
func (loggerStub) Errorf(string, ...interface{})       {}
func (loggerStub) Fatalf(string, ...interface{})       {}
func (l loggerStub) With(...interface{}) logger.Logger { return l }
func (loggerStub) Sync() error                         { return nil }

func TestPublisherMarksSuccessfulEventPublished(t *testing.T) {
	repo := &outboxRepositoryStub{events: []outbox.Event{{ID: "event-1", Topic: "events"}}}
	producer := &producerStub{}
	publisher := NewPublisher(repo, producer, loggerStub{}, time.Second, 10, 3, "events.dlq")
	if err := publisher.publishBatch(context.Background()); err != nil {
		t.Fatalf("publishBatch() error = %v", err)
	}
	if len(repo.publishedIDs) != 1 || repo.publishedIDs[0] != "event-1" || len(repo.failedIDs) != 0 {
		t.Fatalf("unexpected repository state: %+v", repo)
	}
}

func TestPublisherRetriesFailureAndRoutesExhaustedEventToDLQ(t *testing.T) {
	repo := &outboxRepositoryStub{events: []outbox.Event{{ID: "event-1", Topic: "events", Attempts: 4}}}
	producer := &producerStub{err: errors.New("kafka unavailable")}
	publisher := NewPublisher(repo, producer, loggerStub{}, time.Second, 10, 3, "events.dlq")
	if err := publisher.publishBatch(context.Background()); err != nil {
		t.Fatalf("publishBatch() error = %v", err)
	}
	if len(producer.events) != 1 || producer.events[0].Topic != "events.dlq" {
		t.Fatalf("published events = %+v, want DLQ topic", producer.events)
	}
	if len(repo.failedIDs) != 1 || len(repo.publishedIDs) != 0 {
		t.Fatalf("unexpected repository state: %+v", repo)
	}
}
