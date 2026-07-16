package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// TestEncryptedFileServiceOwnerDownload 验证只有当前租户所有者的 AVAILABLE 对象可流式下载并记录摘要审计。
func TestEncryptedFileServiceOwnerDownload(t *testing.T) {
	aggregate := repository.EncryptionTaskAggregate{File: domain.EncryptedFile{ID: 1, PublicID: "file", TenantID: 3, OwnerUserID: 7, OriginalFilename: "demo.txt", Status: domain.EncryptedFileAvailable}, Task: domain.EncryptionTask{ID: 2, PublicID: "task", TenantID: 3, OwnerUserID: 7}}
	object := domain.CiphertextObject{ID: 3, PublicID: "upload", TenantID: 3, ObjectKey: "3/object.cipher", CiphertextSize: 6, CiphertextSHA256: strings.Repeat("a", 64), Status: domain.CiphertextAvailable}
	repositoryLayer := &encryptionRepositoryStub{aggregate: aggregate, object: &object}
	storageLayer := &encryptedStorageStub{bytes: []byte("cipher")}
	audit := &auditRecorderStub{}
	serviceLayer := NewEncryptedFileService(repositoryLayer, storageLayer, audit)
	download, err := serviceLayer.Download(context.Background(), 3, 7, "file")
	if err != nil {
		t.Fatal(err)
	}
	value, _ := io.ReadAll(download.Reader)
	_ = download.Reader.Close()
	if string(value) != "cipher" || download.SHA256 != object.CiphertextSHA256 {
		t.Fatalf("download mismatch: %+v", download)
	}
	if _, err := serviceLayer.Download(context.Background(), 3, 8, "file"); err != response.ErrEncryptedFileNotFound {
		t.Fatalf("cross-owner error=%v", err)
	}
	if len(audit.events) != 2 || audit.events[1].Result != "DENIED" {
		t.Fatalf("download audits=%+v", audit.events)
	}
}

// TestEncryptedFileServiceRejectsUnavailableObject 验证草稿、失败或缺失对象不可下载。
func TestEncryptedFileServiceRejectsUnavailableObject(t *testing.T) {
	repositoryLayer := &encryptionRepositoryStub{aggregate: repository.EncryptionTaskAggregate{File: domain.EncryptedFile{PublicID: "file", TenantID: 3, OwnerUserID: 7, Status: domain.EncryptedFileDraft}}}
	_, err := NewEncryptedFileService(repositoryLayer, &encryptedStorageStub{}, &auditRecorderStub{}).Download(context.Background(), 3, 7, "file")
	if err != response.ErrEncryptedFileUnavailable {
		t.Fatalf("unavailable error=%v", err)
	}
}

// TestFileCenterTenantMemberCanDownloadAndReadEnvelope 验证文件可见用户可获取密文和受保护信封，但不会得到私钥或明文 DEK。
func TestFileCenterTenantMemberCanDownloadAndReadEnvelope(t *testing.T) {
	repositoryLayer := &encryptionRepositoryStub{receivedUserID: 9}
	repositoryLayer.aggregate.File = domain.EncryptedFile{ID: 1, PublicID: "file", TenantID: 3, OwnerUserID: 7, OriginalFilename: "demo.txt", Status: domain.EncryptedFileAvailable}
	repositoryLayer.object = &domain.CiphertextObject{ObjectKey: "3/object.cipher", CiphertextSize: 6, CiphertextSHA256: strings.Repeat("d", 64), Status: domain.CiphertextAvailable}
	repositoryLayer.protectedKey = &domain.ProtectedKey{PublicID: "dek", AlgorithmCode: "RSA-OAEP-SHA256", AlgorithmVersion: "1", ProtectedKeyFormat: "RSA-OAEP-SHA256-RAW", ProtectedKeyBytes: []byte("sealed"), ContextSHA256: strings.Repeat("e", 64)}
	repositoryLayer.rsaBinding = &domain.RSAProtectedKeyBinding{PublicKeyFingerprintSHA256: strings.Repeat("f", 64), OAEPHash: "SHA-256"}
	repositoryLayer.rsaPublicKey = &domain.RSAPublicKey{PublicID: "key", UserID: 7}
	storageLayer := &encryptedStorageStub{bytes: []byte("cipher")}
	serviceLayer := NewEncryptedFileService(repositoryLayer, storageLayer, NoopAuditRecorder{})
	download, err := serviceLayer.DownloadFileCenter(context.Background(), 3, 10, "file")
	if err != nil {
		t.Fatalf("tenant member should download ciphertext: %v", err)
	}
	defer download.Reader.Close()
	if value, _ := io.ReadAll(download.Reader); string(value) != "cipher" {
		t.Fatalf("unexpected ciphertext %q", value)
	}
	if material, err := serviceLayer.OwnDecryptionMaterial(context.Background(), 3, 10, "file"); err != nil || material.ProtectedKeyBase64 == "" {
		t.Fatalf("visible tenant member should receive encrypted envelope: material=%+v err=%v", material, err)
	}
}
