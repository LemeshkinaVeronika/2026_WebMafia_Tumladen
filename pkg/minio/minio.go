package minio

import (
	"context"
	"fmt"
	"io"
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

type ObjectInfo struct {
	Bucket      string
	ObjectName  string
	Reader      io.Reader
	Size        int64
	ContentType string
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

func (c *Client) Upload(ctx context.Context, obj ObjectInfo) (string, error) {
	bucket := obj.Bucket
	if bucket == "" {
		bucket = c.bucket
	}

	objectName := obj.ObjectName
	if objectName == "" {
		objectName = fmt.Sprintf("avatars/%d", time.Now().UnixNano())
	}

	_, err := c.client.PutObject(ctx, bucket, objectName, obj.Reader, obj.Size, minioSDK.PutObjectOptions{
		ContentType: obj.ContentType,
	})
	if err != nil {
		return "", err
	}

	return objectName, nil
}

func (c *Client) Remove(ctx context.Context, bucket, objectName string) error {
	if bucket == "" {
		bucket = c.bucket
	}
	return c.client.RemoveObject(ctx, bucket, objectName, minioSDK.RemoveObjectOptions{})
}

func (c *Client) GetFileURL(ctx context.Context, bucket, objectName string) (string, error) {
	if bucket == "" {
		bucket = c.bucket
	}
	u, err := c.client.PresignedGetObject(ctx, bucket, objectName, time.Hour, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
