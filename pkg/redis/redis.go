package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/rueidis"
)

type Config struct {
	URL       string
	KeyPrefix string
	Timeout   time.Duration
}

func New(ctx context.Context, cfg Config) (rueidis.Client, error) {
	options, err := rueidis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	options.ClientName = "tumladan-backend"
	options.DisableCache = true

	client, err := rueidis.NewClient(options)
	if err != nil {
		return nil, fmt.Errorf("create redis client: %w", err)
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := client.Do(pingCtx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
