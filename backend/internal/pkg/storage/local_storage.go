package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage 使用本地磁盘保存上传文件，并生成面向前端的 URL。
type LocalStorage struct {
	rootDir   string
	urlPrefix string
}

// NewLocalStorage 创建本地文件存储，rootDir 是磁盘根目录，urlPrefix 是对外访问前缀。
func NewLocalStorage(rootDir, urlPrefix string) *LocalStorage {
	return &LocalStorage{rootDir: rootDir, urlPrefix: strings.TrimRight(urlPrefix, "/")}
}

// SaveAvatar 保存用户头像文件，并返回前端可访问 URL 与内部对象键。
func (s *LocalStorage) SaveAvatar(_ context.Context, userID uint64, filename string, _ string, reader io.Reader) (UploadResult, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	random, err := randomName(6)
	if err != nil {
		return UploadResult{}, err
	}
	relDir := filepath.Join("avatars", uintToString(userID))
	// 文件名使用服务端生成的时间戳和随机后缀，不信任用户上传的原始文件名，降低路径穿越和覆盖风险。
	name := time.Now().UTC().Format("20060102150405") + "_" + random + ext
	objectKey := filepath.ToSlash(filepath.Join(relDir, name))
	fullDir := filepath.Join(s.rootDir, relDir)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return UploadResult{}, err
	}
	fullPath := filepath.Join(fullDir, name)
	dst, err := os.Create(fullPath)
	if err != nil {
		return UploadResult{}, err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, reader); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{URL: s.urlPrefix + "/" + objectKey, ObjectKey: objectKey}, nil
}

// Delete 删除指定对象键对应的本地文件，objectKey 为空时不执行操作。
func (s *LocalStorage) Delete(_ context.Context, objectKey string) error {
	if objectKey == "" {
		return nil
	}
	return os.Remove(filepath.Join(s.rootDir, filepath.FromSlash(objectKey)))
}

// randomName 生成头像文件名中的随机片段，降低同秒上传时的覆盖风险。
func randomName(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// uintToString 将 uint64 转为十进制字符串，避免引入额外格式化依赖。
func uintToString(id uint64) string {
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}
