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

type LocalStorage struct {
	rootDir   string
	urlPrefix string
}

func NewLocalStorage(rootDir, urlPrefix string) *LocalStorage {
	return &LocalStorage{rootDir: rootDir, urlPrefix: strings.TrimRight(urlPrefix, "/")}
}

func (s *LocalStorage) SaveAvatar(_ context.Context, userID uint64, filename string, _ string, reader io.Reader) (UploadResult, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	random, err := randomName(6)
	if err != nil {
		return UploadResult{}, err
	}
	relDir := filepath.Join("avatars", uintToString(userID))
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

func (s *LocalStorage) Delete(_ context.Context, objectKey string) error {
	if objectKey == "" {
		return nil
	}
	return os.Remove(filepath.Join(s.rootDir, filepath.FromSlash(objectKey)))
}

func randomName(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

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
