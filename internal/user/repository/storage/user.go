package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/webmafia/tumladan/pkg/minio"
)

func (s *Storage) UploadAvatar(ctx context.Context, file io.Reader, _ string, size int64, contentType string) (string, error) {
	obj := minio.ObjectInfo{
		Bucket:      s.bucket,
		ObjectName:  avatarObjectName(contentType),
		Reader:      file,
		Size:        size,
		ContentType: contentType,
	}

	objectName, err := s.client.Upload(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("upload avatar: %w", ErrUploadFailed)
	}
	return objectName, nil
}

func (s *Storage) DeleteAvatar(ctx context.Context, objectName string) error {
	if err := s.client.Remove(ctx, s.bucket, objectName); err != nil {
		return fmt.Errorf("delete avatar: %w", ErrDeleteFailed)
	}
	return nil
}

func (s *Storage) GetAvatarURL(ctx context.Context, objectName string) (string, error) {
	url, err := s.client.GetFileURL(ctx, s.bucket, objectName)
	if err != nil {
		return "", fmt.Errorf("get avatar url: %w", ErrURLFailed)
	}
	return url, nil
}

func avatarObjectName(contentType string) string {
	ext := strings.TrimPrefix(strings.TrimSpace(contentType), "image/")
	return fmt.Sprintf("%s.%s", uuid.NewString(), ext)
}
