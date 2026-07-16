package storage

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalEncryptedFileStorage 在受控本地目录保存密文，所有读取仍需经过业务鉴权。
type LocalEncryptedFileStorage struct {
	rootDir string
	tempDir string
}

// NewLocalEncryptedFileStorage 创建本地密文存储；两个目录都不得注册为静态资源。
func NewLocalEncryptedFileStorage(rootDir, tempDir string) *LocalEncryptedFileStorage {
	return &LocalEncryptedFileStorage{rootDir: filepath.Clean(rootDir), tempDir: filepath.Clean(tempDir)}
}

// StageCiphertext 流式写入暂存对象并由服务端复核字节数和 SHA-256。
func (s *LocalEncryptedFileStorage) StageCiphertext(ctx context.Context, tenantID uint64, attemptPublicID string, reader io.Reader, maxBytes int64, expectedSHA256 string) (CiphertextUploadResult, error) {
	if tenantID == 0 || attemptPublicID == "" || reader == nil || maxBytes <= 0 {
		return CiphertextUploadResult{}, errors.New("invalid ciphertext upload input")
	}
	random, err := secureObjectName()
	if err != nil {
		return CiphertextUploadResult{}, err
	}
	relative := filepath.Join(fmt.Sprintf("%d", tenantID), attemptPublicID, random+".part")
	fullPath, err := safeJoin(s.tempDir, relative)
	if err != nil {
		return CiphertextUploadResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
		return CiphertextUploadResult{}, err
	}
	file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return CiphertextUploadResult{}, err
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(fullPath)
		}
	}()
	hasher := sha256.New()
	limited := io.LimitReader(&contextReader{ctx: ctx, reader: reader}, maxBytes+1)
	written, err := io.Copy(io.MultiWriter(file, hasher), limited)
	if err != nil {
		return CiphertextUploadResult{}, err
	}
	if written <= 0 || written > maxBytes {
		return CiphertextUploadResult{}, errors.New("ciphertext exceeds size limit")
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA256 != "" && !strings.EqualFold(expectedSHA256, actual) {
		return CiphertextUploadResult{}, errors.New("ciphertext hash mismatch")
	}
	if err := file.Sync(); err != nil {
		return CiphertextUploadResult{}, err
	}
	committed = true
	return CiphertextUploadResult{ObjectKey: filepath.ToSlash(filepath.Join(".staging", relative)), Size: written, SHA256: actual}, nil
}

// CommitCiphertext 将暂存对象原子移动到正式目录并返回新的内部对象键。
func (s *LocalEncryptedFileStorage) CommitCiphertext(_ context.Context, stagingObjectKey string) (string, error) {
	const prefix = ".staging/"
	if !strings.HasPrefix(filepath.ToSlash(stagingObjectKey), prefix) {
		return "", errors.New("object is not staging ciphertext")
	}
	relative := strings.TrimPrefix(filepath.ToSlash(stagingObjectKey), prefix)
	stagingPath, err := safeJoin(s.tempDir, filepath.FromSlash(relative))
	if err != nil {
		return "", err
	}
	random, err := secureObjectName()
	if err != nil {
		return "", err
	}
	parts := strings.Split(relative, "/")
	if len(parts) < 1 || parts[0] == "" {
		return "", errors.New("invalid staging object tenant")
	}
	finalKey := filepath.ToSlash(filepath.Join(parts[0], random+".cipher"))
	finalPath, err := safeJoin(s.rootDir, filepath.FromSlash(finalKey))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(finalPath), 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(stagingPath, finalPath); err != nil {
		return "", err
	}
	return finalKey, nil
}

// OpenCiphertext 打开正式密文对象；调用方必须先完成租户、所有者和状态鉴权。
func (s *LocalEncryptedFileStorage) OpenCiphertext(_ context.Context, objectKey string) (io.ReadCloser, error) {
	if strings.HasPrefix(filepath.ToSlash(objectKey), ".staging/") {
		return nil, errors.New("staging ciphertext is not downloadable")
	}
	fullPath, err := safeJoin(s.rootDir, filepath.FromSlash(objectKey))
	if err != nil {
		return nil, err
	}
	return os.Open(fullPath)
}

// DeleteCiphertext 幂等删除正式或暂存密文，不允许对象键逃逸受控目录。
func (s *LocalEncryptedFileStorage) DeleteCiphertext(_ context.Context, objectKey string) error {
	base := s.rootDir
	relative := objectKey
	if strings.HasPrefix(filepath.ToSlash(objectKey), ".staging/") {
		base = s.tempDir
		relative = strings.TrimPrefix(filepath.ToSlash(objectKey), ".staging/")
	}
	fullPath, err := safeJoin(base, filepath.FromSlash(relative))
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// DeleteExpiredStaging 删除截止时间前的受控 .part 文件；目录遍历不会跟随符号链接。
func (s *LocalEncryptedFileStorage) DeleteExpiredStaging(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	deleted := 0
	err := filepath.WalkDir(s.tempDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if deleted >= limit {
			return filepath.SkipAll
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".part") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		deleted++
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return deleted, nil
	}
	return deleted, err
}

// contextReader 在上传取消后停止继续读取请求体。
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

// Read 在每次底层读取前检查请求上下文。
func (r *contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}

// secureObjectName 生成不可预测对象名，避免使用原始文件名或可枚举 ID。
func secureObjectName() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

// safeJoin 验证清理后的目标仍位于预期根目录内，防止路径穿越。
func safeJoin(root, relative string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(rootAbs, relative))
	if err != nil {
		return "", err
	}
	prefix := rootAbs + string(os.PathSeparator)
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, prefix) {
		return "", errors.New("object path escapes storage root")
	}
	return targetAbs, nil
}
