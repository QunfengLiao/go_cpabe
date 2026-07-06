package storage

import (
	"context"
	"io"
)

type UploadResult struct {
	URL       string
	ObjectKey string
}

type Storage interface {
	SaveAvatar(ctx context.Context, userID uint64, filename string, contentType string, reader io.Reader) (UploadResult, error)
	Delete(ctx context.Context, objectKey string) error
}
