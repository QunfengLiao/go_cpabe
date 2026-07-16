package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

// TestLargeCiphertextStreamingUpload 验证大于内部复制缓冲区的密文以流式方式暂存并保持摘要一致。
func TestLargeCiphertextStreamingUpload(t *testing.T) {
	payload := bytes.Repeat([]byte("streaming-ciphertext"), 512*1024)
	sum := sha256.Sum256(payload)
	storageLayer := NewLocalEncryptedFileStorage(t.TempDir(), t.TempDir())
	result, err := storageLayer.StageCiphertext(context.Background(), 3, "attempt", bytes.NewReader(payload), int64(len(payload)), hex.EncodeToString(sum[:]))
	if err != nil {
		t.Fatal(err)
	}
	if result.Size != int64(len(payload)) || result.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("stream result=%+v", result)
	}
	finalKey, err := storageLayer.CommitCiphertext(context.Background(), result.ObjectKey)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := storageLayer.OpenCiphertext(context.Background(), finalKey)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	actual, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, payload) {
		t.Fatal("streaming ciphertext changed")
	}
}

// TestCiphertextUploadRejectsLimitAndDigestMismatch 验证超过声明上限或摘要不一致不会留下可提交暂存对象。
func TestCiphertextUploadRejectsLimitAndDigestMismatch(t *testing.T) {
	storageLayer := NewLocalEncryptedFileStorage(t.TempDir(), t.TempDir())
	if _, err := storageLayer.StageCiphertext(context.Background(), 3, "attempt", bytes.NewReader([]byte("too-large")), 3, ""); err == nil {
		t.Fatal("size limit must fail")
	}
	if _, err := storageLayer.StageCiphertext(context.Background(), 3, "attempt", bytes.NewReader([]byte("cipher")), 6, "bad-digest"); err == nil {
		t.Fatal("digest mismatch must fail")
	}
}
