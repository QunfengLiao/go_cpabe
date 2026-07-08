package storage

import (
	"context"
	"io"
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
