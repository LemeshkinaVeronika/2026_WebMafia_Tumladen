package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
	analyticsPostgres "github.com/webmafia/tumladan/internal/analytics/repository/postgres"
	analyticsService "github.com/webmafia/tumladan/internal/analytics/service"
	outboxPostgres "github.com/webmafia/tumladan/internal/outbox/repository/postgres"
	outboxService "github.com/webmafia/tumladan/internal/outbox/service"
	kafkaclient "github.com/webmafia/tumladan/pkg/kafka"
	"github.com/webmafia/tumladan/pkg/logger"
	"github.com/webmafia/tumladan/pkg/postgres"
)

type Worker struct {
	config    Config
	logger    logger.Logger
	db        *sql.DB
	producer  *kafkaclient.Producer
	consumer  *kgo.Client
	publisher *outboxService.Publisher
	handler   *analyticsService.Consumer
}

func New(ctx context.Context, config Config) (*Worker, error) {
	logMode := logger.ModeProd
	if config.AppEnv == "local" {
		logMode = logger.ModeDev
	}
	log, err := logger.New(config.LogLevel, logMode)
	if err != nil {
		return nil, err
	}
	db, err := postgres.New(ctx, config.Postgres)
	if err != nil {
		_ = log.Sync()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	producer, err := kafkaclient.NewProducer(ctx, config.KafkaBrokers)
	if err != nil {
		_ = db.Close()
		_ = log.Sync()
		return nil, err
	}
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(config.KafkaBrokers...),
		kgo.ConsumerGroup(config.ConsumerGroup),
		kgo.ConsumeTopics(config.Topic),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
	)
	if err != nil {
		producer.Close()
		_ = db.Close()
		_ = log.Sync()
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}

	return &Worker{
		config:    config,
		logger:    log,
		db:        db,
		producer:  producer,
		consumer:  consumer,
		publisher: outboxService.NewPublisher(outboxPostgres.New(db), producer, log, config.PollInterval, config.BatchSize, config.MaxAttempts, config.DLQTopic),
		handler:   analyticsService.NewConsumer(analyticsPostgres.New(db), config.ConsumerGroup),
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.logger.Infof("starting event worker brokers=%v topic=%s group=%s", w.config.KafkaBrokers, w.config.Topic, w.config.ConsumerGroup)
	defer func() {
		cancel()
		w.Close()
	}()

	errCh := make(chan error, 2)
	go func() { errCh <- w.publisher.Run(ctx) }()
	go func() { errCh <- w.consume(ctx) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}

func (w *Worker) consume(ctx context.Context) error {
	for {
		fetches := w.consumer.PollRecords(ctx, w.config.BatchSize)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if fetchErrors := fetches.Errors(); len(fetchErrors) > 0 {
			return fmt.Errorf("poll kafka: %v", fetchErrors)
		}

		for _, record := range fetches.Records() {
			applied, err := w.handler.Handle(ctx, record.Value)
			if err != nil {
				w.consumer.AllowRebalance()
				return fmt.Errorf("handle kafka record topic=%s partition=%d offset=%d: %w", record.Topic, record.Partition, record.Offset, err)
			}
			if err := w.consumer.CommitRecords(ctx, record); err != nil {
				w.consumer.AllowRebalance()
				return fmt.Errorf("commit kafka offset: %w", err)
			}
			w.logger.With("event_key", string(record.Key), "applied", applied).Debugf("match event processed")
		}
		w.consumer.AllowRebalance()
	}
}

func (w *Worker) Close() {
	w.consumer.Close()
	w.producer.Close()
	_ = w.db.Close()
	_ = w.logger.Sync()
}
