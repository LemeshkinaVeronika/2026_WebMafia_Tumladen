package minio

import (
	"context"
	"fmt"
	"io"
	"time"

	minioSDK "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/cors"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint     string
	AccessKey    string
	SecretKey    string
	Bucket       string
	AvatarBucket string
	UseSSL       bool
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

	buckets := []string{cfg.Bucket}
	if cfg.AvatarBucket != "" && cfg.AvatarBucket != cfg.Bucket {
		buckets = append(buckets, cfg.AvatarBucket)
	}

	var lastErr error
	for attempt := 0; attempt < 30; attempt++ {
		if err := result.ensurePublicBuckets(ctx, buckets); err == nil {
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
	return c.EnsurePublicBucket(ctx, c.bucket)
}

func (c *Client) EnsurePublicBucket(ctx context.Context, bucket string) error {
	return c.ensureBucket(ctx, bucket, true)
}

func (c *Client) ensurePublicBuckets(ctx context.Context, buckets []string) error {
	for _, bucket := range buckets {
		if err := c.EnsurePublicBucket(ctx, bucket); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ensureBucket(ctx context.Context, bucket string, publicRead bool) error {
	exists, err := c.client.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("check minio bucket %q: %w", bucket, err)
	}

	if !exists {
		if err := c.client.MakeBucket(ctx, bucket, minioSDK.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create minio bucket %q: %w", bucket, err)
		}
	}

	if publicRead {
		if err := c.client.SetBucketPolicy(ctx, bucket, publicReadPolicy(bucket)); err != nil {
			return fmt.Errorf("set minio bucket %q public read policy: %w", bucket, err)
		}
		if err := c.client.SetBucketCors(ctx, bucket, publicReadCORS()); err != nil {
			if minioSDK.ToErrorResponse(err).Code == minioSDK.NotImplemented {
				return nil
			}
			return fmt.Errorf("set minio bucket %q cors: %w", bucket, err)
		}
	}

	return nil
}

func publicReadPolicy(bucket string) string {
	return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
}

func publicReadCORS() *cors.Config {
	return &cors.Config{
		CORSRules: []cors.Rule{{
			AllowedOrigin: []string{"*"},
			AllowedMethod: []string{"GET", "HEAD"},
			AllowedHeader: []string{"*"},
			MaxAgeSeconds: 3600,
		}},
	}
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
