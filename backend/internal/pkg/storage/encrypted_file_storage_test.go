package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

// TestEncryptedFileStorageLifecycle 验证暂存、摘要复核、提交、流式读取和幂等删除闭环。
func TestEncryptedFileStorageLifecycle(t *testing.T) {
	root, temporary := t.TempDir(), t.TempDir()
	storage := NewLocalEncryptedFileStorage(root, temporary)
	payload := bytes.Repeat([]byte("ciphertext"), 1024)
	sum := sha256.Sum256(payload)
	staged, err := storage.StageCiphertext(context.Background(), 7, "attempt-id", bytes.NewReader(payload), int64(len(payload)), hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(staged.ObjectKey, ".staging/") {
		t.Fatalf("unexpected staging key: %s", staged.ObjectKey)
	}
	if _, err := storage.OpenCiphertext(context.Background(), staged.ObjectKey); err == nil {
		t.Fatal("staging object must not be downloadable")
	}
	finalKey, err := storage.CommitCiphertext(context.Background(), staged.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := storage.OpenCiphertext(context.Background(), finalKey)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, payload) {
		t.Fatal("ciphertext changed")
	}
	if err := storage.DeleteCiphertext(context.Background(), finalKey); err != nil {
		t.Fatal(err)
	}
	if err := storage.DeleteCiphertext(context.Background(), finalKey); err != nil {
		t.Fatal(err)
	}
}

// TestEncryptedFileStorageRejectsHashAndTraversal 验证摘要不一致和对象键路径穿越被拒绝。
func TestEncryptedFileStorageRejectsHashAndTraversal(t *testing.T) {
	storage := NewLocalEncryptedFileStorage(t.TempDir(), t.TempDir())
	if _, err := storage.StageCiphertext(context.Background(), 1, "attempt", bytes.NewReader([]byte("x")), 1, strings.Repeat("0", 64)); err == nil {
		t.Fatal("hash mismatch must fail")
	}
	if _, err := storage.OpenCiphertext(context.Background(), "../secret"); err == nil {
		t.Fatal("path traversal must fail")
	}
}
