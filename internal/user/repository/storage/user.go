package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/webmafia/tumladan/pkg/minio"
)

func (s *Storage) UploadAvatar(ctx context.Context, file io.Reader, filename string, size int64, contentType string) (string, error) {
	obj := minio.ObjectInfo{
		Bucket:      s.bucket,
		ObjectName:  cleanAvatarFilename(filename),
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

func cleanAvatarFilename(filename string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == string(filepath.Separator) {
		return ""
	}
	return filename
}
