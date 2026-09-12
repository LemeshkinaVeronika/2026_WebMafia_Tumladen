package kafka

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/webmafia/tumladan/internal/outbox"
)

type Producer struct {
	client *kgo.Client
}

func NewProducer(ctx context.Context, brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping kafka: %w", err)
	}
	return &Producer{client: client}, nil
}

func (p *Producer) Publish(ctx context.Context, event outbox.Event) error {
	record := &kgo.Record{
		Topic: event.Topic,
		Key:   []byte(event.Key),
		Value: event.Payload,
		Headers: []kgo.RecordHeader{
			{Key: "event_id", Value: []byte(event.ID)},
			{Key: "event_type", Value: []byte(event.Type)},
		},
	}
	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("publish kafka event %s: %w", event.ID, err)
	}
	return nil
}

func (p *Producer) Close() {
	p.client.Close()
}
