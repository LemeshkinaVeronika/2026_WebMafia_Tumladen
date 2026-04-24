package minio

import (
	"context"
	"fmt"
	"time"

	minioSDK "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

type Client struct {
	client *minioSDK.Client
	bucket string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	client, err := minioSDK.New(cfg.Endpoint, &minioSDK.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	result := &Client{
		client: client,
		bucket: cfg.Bucket,
	}

	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		if err := result.EnsureBucket(ctx); err == nil {
			return result, nil
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}

	return nil, lastErr
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.client.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket %q: %w", c.bucket, err)
	}

	if !exists {
		if err := c.client.MakeBucket(ctx, c.bucket, minioSDK.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create minio bucket %q: %w", c.bucket, err)
		}
	}

	return nil
}
