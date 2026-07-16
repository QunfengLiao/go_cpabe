package storage

import (
	"context"
	"io"
	"time"
)

// UploadResult 描述文件保存后的外部访问地址和内部对象键。
type UploadResult struct {
	URL       string
	ObjectKey string
}

// Storage 定义用户文件存储能力，当前主要用于头像上传。
type Storage interface {
	SaveAvatar(ctx context.Context, userID uint64, filename string, contentType string, reader io.Reader) (UploadResult, error)
	Delete(ctx context.Context, objectKey string) error
}

// CiphertextUploadResult 描述服务端实际保存的暂存密文，不包含公开 URL。
type CiphertextUploadResult struct {
	ObjectKey string
	Size      int64
	SHA256    string
}

// EncryptedFileStorage 定义密文暂存、提交、鉴权读取和补偿删除能力。
type EncryptedFileStorage interface {
	StageCiphertext(ctx context.Context, tenantID uint64, attemptPublicID string, reader io.Reader, maxBytes int64, expectedSHA256 string) (CiphertextUploadResult, error)
	CommitCiphertext(ctx context.Context, stagingObjectKey string) (string, error)
	OpenCiphertext(ctx context.Context, objectKey string) (io.ReadCloser, error)
	DeleteCiphertext(ctx context.Context, objectKey string) error
	DeleteExpiredStaging(ctx context.Context, cutoff time.Time, limit int) (int, error)
}
