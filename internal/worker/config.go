package worker

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/webmafia/tumladan/internal/event"
	"github.com/webmafia/tumladan/pkg/postgres"
)

type Config struct {
	AppEnv        string
	LogLevel      string
	Postgres      postgres.Config
	KafkaBrokers  []string
	Topic         string
	DLQTopic      string
	ConsumerGroup string
	PollInterval  time.Duration
	BatchSize     int
	MaxAttempts   int
}

func LoadConfig() Config {
	topic := env("KAFKA_MATCH_EVENTS_TOPIC", event.MatchEventsTopic)
	return Config{
		AppEnv:   env("APP_ENV", "local"),
		LogLevel: env("LOG_LEVEL", "info"),
		Postgres: postgres.Config{
			Host:            env("DB_HOST", "postgres"),
			Port:            envInt("DB_PORT", 5432),
			User:            env("DB_USER", "webmafia"),
			Password:        env("DB_PASSWORD", "webmafia"),
			DBName:          env("DB_NAME", "webmafia"),
			SSLMode:         env("DB_SSLMODE", "disable"),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		KafkaBrokers:  envCSV("KAFKA_BROKERS", []string{"localhost:9092"}),
		Topic:         topic,
		DLQTopic:      env("KAFKA_MATCH_EVENTS_DLQ_TOPIC", topic+".dlq"),
		ConsumerGroup: env("KAFKA_CONSUMER_GROUP", "tumladan-match-projections-v1"),
		PollInterval:  envDuration("OUTBOX_POLL_INTERVAL", 500*time.Millisecond),
		BatchSize:     envInt("OUTBOX_BATCH_SIZE", 100),
		MaxAttempts:   envInt("OUTBOX_MAX_ATTEMPTS", 10),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
