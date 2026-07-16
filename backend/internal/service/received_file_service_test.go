package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"go-cpabe/backend/internal/domain"
)

// TestReceivedFileServiceReturnsAllVisibleKeyEnvelopes 验证可见用户都能取得受保护信封，但服务端不返回私钥或明文 DEK。
func TestReceivedFileServiceReturnsAllVisibleKeyEnvelopes(t *testing.T) {
	repositoryLayer := &encryptionRepositoryStub{receivedUserID: 9}
	repositoryLayer.aggregate.File = domain.EncryptedFile{ID: 1, PublicID: "file", TenantID: 3, OwnerUserID: 7, OriginalFilename: "demo.txt", PlaintextSize: 6, Status: domain.EncryptedFileAvailable}
	repositoryLayer.protectedKey = &domain.ProtectedKey{AlgorithmCode: "RSA-OAEP-SHA256", AlgorithmVersion: "1", ProtectedKeyFormat: "RSA-OAEP-SHA256-RAW", ProtectedKeyBytes: []byte("protected"), ContextSHA256: strings.Repeat("a", 64)}
	repositoryLayer.rsaBinding = &domain.RSAProtectedKeyBinding{RecipientUserID: 9, PublicKeyFingerprintSHA256: strings.Repeat("b", 64)}
	repositoryLayer.rsaPublicKey = &domain.RSAPublicKey{PublicID: "key-uuid", UserID: 9}
	serviceLayer := NewEncryptedFileService(repositoryLayer, &encryptedStorageStub{}, NoopAuditRecorder{})
	material, err := serviceLayer.ReceivedMaterial(context.Background(), 3, 9, "file")
	if err != nil || material.RSAPublicKeyID != "key-uuid" || material.ProtectedKeyBase64 == "" {
		t.Fatalf("material=%+v err=%v", material, err)
	}
	if _, err := serviceLayer.ReceivedMaterial(context.Background(), 3, 10, "file"); err != nil {
		t.Fatalf("visible user should not be rejected by recipient RBAC: %v", err)
	}
}

// TestReceivedFileServiceStreamsAuthorizedCiphertext 验证接收者下载复用受控存储流而不暴露对象键。
func TestReceivedFileServiceStreamsAuthorizedCiphertext(t *testing.T) {
	repositoryLayer := &encryptionRepositoryStub{receivedUserID: 9}
	repositoryLayer.aggregate.File = domain.EncryptedFile{ID: 1, PublicID: "file", TenantID: 3, OriginalFilename: "demo.txt", Status: domain.EncryptedFileAvailable}
	repositoryLayer.object = &domain.CiphertextObject{ObjectKey: "secret/object", CiphertextSize: 6, CiphertextSHA256: strings.Repeat("c", 64), Status: domain.CiphertextAvailable}
	storageLayer := &encryptedStorageStub{bytes: []byte("cipher")}
	download, err := NewEncryptedFileService(repositoryLayer, storageLayer, NoopAuditRecorder{}).DownloadReceived(context.Background(), 3, 9, "file")
	if err != nil {
		t.Fatal(err)
	}
	defer download.Reader.Close()
	value, _ := io.ReadAll(download.Reader)
	if string(value) != "cipher" || download.SHA256 == "" {
		t.Fatalf("download=%+v value=%q", download, value)
	}
}
