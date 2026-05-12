package storage

import (
	"github.com/webmafia/tumladan/pkg/minio"
)

type Storage struct {
	client *minio.Client
	bucket string
}

func NewStorage(client *minio.Client, bucket string) *Storage {
	return &Storage{
		client: client,
		bucket: bucket,
	}
}

func New(client *minio.Client, bucket string) *Storage {
	return NewStorage(client, bucket)
}
