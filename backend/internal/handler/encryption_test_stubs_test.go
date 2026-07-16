package handler

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// encryptionApplicationStub 为加密 Handler 契约测试提供可观察的最小业务替身。
type encryptionApplicationStub struct {
	lastCreate   service.CreateEncryptionTaskInput
	lastProgress service.ProgressInput
	lastUpload   []byte
}

// Algorithms 返回首期 RSA 算法能力。
func (s *encryptionApplicationStub) Algorithms(context.Context, uint64) ([]domain.EncryptionAlgorithm, error) {
	return []domain.EncryptionAlgorithm{{Code: "RSA-OAEP-SHA256", Version: "1", DisplayName: "RSA + AES", AuthorizationType: "RSA_RECIPIENT", Status: "ACTIVE"}}, nil
}

// CreateTask 返回固定外部 UUID 聚合，验证 Handler 不泄漏内部主键。
func (s *encryptionApplicationStub) CreateTask(_ context.Context, _, _ uint64, _ string, input service.CreateEncryptionTaskInput) (repository.EncryptionTaskAggregate, bool, error) {
	s.lastCreate = input
	return encryptionHandlerAggregate(), false, nil
}

// Task 返回固定任务聚合。
func (s *encryptionApplicationStub) Task(context.Context, uint64, uint64, string) (repository.EncryptionTaskAggregate, error) {
	return encryptionHandlerAggregate(), nil
}

// ReportProgress 记录真实进度输入并返回更新后聚合。
func (s *encryptionApplicationStub) ReportProgress(_ context.Context, _, _ uint64, _, _ string, input service.ProgressInput) (repository.EncryptionTaskAggregate, error) {
	s.lastProgress = input
	result := encryptionHandlerAggregate()
	result.Attempt.Status, result.Attempt.ProcessedBytes = input.Stage, input.ProcessedBytes
	return result, nil
}

// UploadCiphertext 读取流并返回暂存对象元数据。
func (s *encryptionApplicationStub) UploadCiphertext(_ context.Context, _, _ uint64, _, _, expectedSHA256, format string, _ int64, reader io.Reader) (domain.CiphertextObject, error) {
	s.lastUpload, _ = io.ReadAll(reader)
	return domain.CiphertextObject{PublicID: "323e4567-e89b-42d3-a456-426614174000", CiphertextSize: int64(len(s.lastUpload)), CiphertextSHA256: expectedSHA256, ContainerFormat: format, Status: domain.CiphertextStaging}, nil
}

// Complete 返回完成文件聚合。
func (s *encryptionApplicationStub) Complete(context.Context, uint64, uint64, string, string, service.CompleteEncryptionInput) (repository.EncryptionTaskAggregate, bool, error) {
	result := encryptionHandlerAggregate()
	result.File.Status = domain.EncryptedFileAvailable
	return result, false, nil
}

// Cancel 返回取消终态。
func (s *encryptionApplicationStub) Cancel(context.Context, uint64, uint64, string) (repository.EncryptionTaskAggregate, error) {
	result := encryptionHandlerAggregate()
	result.Task.Status, result.Attempt.Status = domain.EncryptionCancelled, domain.EncryptionCancelled
	return result, nil
}

// Retry 返回 attempt_no 递增的新执行。
func (s *encryptionApplicationStub) Retry(context.Context, uint64, uint64, string) (repository.EncryptionTaskAggregate, error) {
	result := encryptionHandlerAggregate()
	result.Attempt.AttemptNo = 2
	result.Attempt.PublicID = "423e4567-e89b-42d3-a456-426614174000"
	return result, nil
}

// Fail 接收脱敏失败上报。
func (s *encryptionApplicationStub) Fail(context.Context, uint64, uint64, string, string, string, bool) error {
	return nil
}

// rsaKeyApplicationStub 返回只含公钥的 RSA 测试数据。
type rsaKeyApplicationStub struct{}

// MyKeys 返回当前成员公钥历史。
func (rsaKeyApplicationStub) MyKeys(context.Context, uint64, uint64) ([]domain.RSAPublicKey, error) {
	return []domain.RSAPublicKey{handlerRSAKey()}, nil
}

// RegisterMyKey 返回新登记公钥。
func (rsaKeyApplicationStub) RegisterMyKey(context.Context, uint64, uint64, service.RegisterRSAKeyInput) (domain.RSAPublicKey, bool, error) {
	return handlerRSAKey(), false, nil
}

// Recipients 返回同租户接收者及公钥。
func (rsaKeyApplicationStub) Recipients(context.Context, uint64, uint64) ([]repository.RSARecipient, error) {
	return []repository.RSARecipient{{UserID: 9, DisplayName: "接收者", Keys: []domain.RSAPublicKey{handlerRSAKey()}}}, nil
}

// UpdateStatus 返回禁用后的公钥。
func (rsaKeyApplicationStub) UpdateStatus(context.Context, uint64, uint64, string, service.UpdateRSAKeyStatusInput) (domain.RSAPublicKey, error) {
	key := handlerRSAKey()
	key.Status = "DISABLED"
	return key, nil
}

// encryptedFileApplicationStub 返回可下载的当前所有者密文。
type encryptedFileApplicationStub struct{}

// ListFileCenter 返回文件中心测试分页，覆盖企业云盘和分享给我统一入口。
func (encryptedFileApplicationStub) ListFileCenter(context.Context, uint64, uint64, service.FileCenterScope, int, int) (repository.EncryptedFilePage, error) {
	return repository.EncryptedFilePage{Items: []repository.FileCenterItem{handlerFileCenterItem(encryptionHandlerAggregate().File)}, Total: 1}, nil
}

// FileCenterDetail 返回统一文件中心公开详情，不携带对象键或受保护密钥字节。
func (encryptedFileApplicationStub) FileCenterDetail(context.Context, uint64, uint64, string) (repository.EncryptedFileDetail, error) {
	aggregate := encryptionHandlerAggregate()
	object := domain.CiphertextObject{CiphertextSize: 6, CiphertextSHA256: strings.Repeat("a", 64), ContainerFormat: "GCPABE01", Status: domain.CiphertextAvailable}
	return repository.EncryptedFileDetail{Summary: handlerFileCenterItem(aggregate.File), File: aggregate.File, Task: aggregate.Task, Object: &object}, nil
}

// DownloadFileCenter 模拟同租户普通成员可下载但未必能解密的密文流。
func (encryptedFileApplicationStub) DownloadFileCenter(context.Context, uint64, uint64, string) (service.EncryptedFileDownload, error) {
	return service.EncryptedFileDownload{Reader: io.NopCloser(strings.NewReader("cipher")), Filename: "demo.txt.gcpabe", Size: 6, SHA256: strings.Repeat("a", 64)}, nil
}

// OwnDecryptionMaterial 模拟可见文件的密钥信封响应。
func (encryptedFileApplicationStub) OwnDecryptionMaterial(context.Context, uint64, uint64, string) (service.ReceivedDecryptionMaterial, error) {
	return service.ReceivedDecryptionMaterial{FileID: "523e4567-e89b-42d3-a456-426614174000", ProtectedKeyBase64: "Y2lwaGVy", ContextSHA256: strings.Repeat("a", 64), RSAPublicKeyID: "623e4567-e89b-42d3-a456-426614174000"}, nil
}

// ListOwned 返回单条自有文件分页。
func (encryptedFileApplicationStub) ListOwned(context.Context, uint64, uint64, domain.EncryptedFileStatus, int, int) (repository.EncryptedFilePage, error) {
	return repository.EncryptedFilePage{Items: []repository.FileCenterItem{handlerFileCenterItem(encryptionHandlerAggregate().File)}, Total: 1}, nil
}

// Detail 返回脱敏文件详情。
func (encryptedFileApplicationStub) Detail(context.Context, uint64, uint64, string) (repository.EncryptedFileDetail, error) {
	aggregate := encryptionHandlerAggregate()
	object := domain.CiphertextObject{CiphertextSize: 6, CiphertextSHA256: strings.Repeat("a", 64), ContainerFormat: "GCPABE01", Status: domain.CiphertextAvailable}
	return repository.EncryptedFileDetail{File: aggregate.File, Task: aggregate.Task, Object: &object}, nil
}

// Download 返回内存密文流及摘要。
func (encryptedFileApplicationStub) Download(context.Context, uint64, uint64, string) (service.EncryptedFileDownload, error) {
	return service.EncryptedFileDownload{Reader: io.NopCloser(strings.NewReader("cipher")), Filename: "demo.txt.gcpabe", Size: 6, SHA256: strings.Repeat("a", 64)}, nil
}

// ListReceived 返回当前接收者单条文件分页。
func (encryptedFileApplicationStub) ListReceived(context.Context, uint64, uint64, int, int) (repository.EncryptedFilePage, error) {
	return repository.EncryptedFilePage{Items: []repository.FileCenterItem{handlerFileCenterItem(encryptionHandlerAggregate().File)}, Total: 1}, nil
}

// ReceivedMaterial 返回不含私钥和明文 DEK 的测试解密材料。
func (encryptedFileApplicationStub) ReceivedMaterial(context.Context, uint64, uint64, string) (service.ReceivedDecryptionMaterial, error) {
	return service.ReceivedDecryptionMaterial{FileID: "523e4567-e89b-42d3-a456-426614174000", ProtectedKeyBase64: "Y2lwaGVy", ContextSHA256: strings.Repeat("a", 64), RSAPublicKeyID: "623e4567-e89b-42d3-a456-426614174000"}, nil
}

// DownloadReceived 返回接收者授权的测试密文流。
func (encryptedFileApplicationStub) DownloadReceived(context.Context, uint64, uint64, string) (service.EncryptedFileDownload, error) {
	return service.EncryptedFileDownload{Reader: io.NopCloser(strings.NewReader("cipher")), Filename: "demo.txt.gcpabe", Size: 6, SHA256: strings.Repeat("a", 64)}, nil
}

// encryptionHandlerAggregate 构造不含秘密材料的固定任务聚合。
func encryptionHandlerAggregate() repository.EncryptionTaskAggregate {
	now := time.Now()
	return repository.EncryptionTaskAggregate{Task: domain.EncryptionTask{ID: 91, PublicID: "123e4567-e89b-42d3-a456-426614174000", FileID: 92, AlgorithmCode: "RSA-OAEP-SHA256", AlgorithmVersion: "1", AuthorizationSnapshot: []byte(`{"type":"RSA_RECIPIENT","recipient_user_id":9}`), AuthorizationSnapshotSHA256: strings.Repeat("b", 64), Status: domain.EncryptionPending, CreatedAt: now}, Attempt: domain.EncryptionTaskAttempt{ID: 93, PublicID: "223e4567-e89b-42d3-a456-426614174000", AttemptNo: 1, Status: domain.EncryptionPending, TotalBytes: 6}, File: domain.EncryptedFile{ID: 92, PublicID: "523e4567-e89b-42d3-a456-426614174000", OriginalFilename: "demo.txt", PlaintextSize: 6, Status: domain.EncryptedFileDraft, CreatedAt: now}}
}

// handlerFileCenterItem 构造 Handler 契约测试使用的文件中心列表 DTO。
func handlerFileCenterItem(file domain.EncryptedFile) repository.FileCenterItem {
	return repository.FileCenterItem{ID: file.PublicID, OriginalFilename: file.OriginalFilename, PlaintextSize: file.PlaintextSize, Status: file.Status, OwnerUserID: file.OwnerUserID, Owner: repository.FileCenterUserSummary{UserID: file.OwnerUserID}, CreatedAt: file.CreatedAt, CompletedAt: file.CompletedAt}
}

// handlerRSAKey 构造绝不含私钥字段的公钥响应。
func handlerRSAKey() domain.RSAPublicKey {
	return domain.RSAPublicKey{PublicID: "623e4567-e89b-42d3-a456-426614174000", UserID: 9, Version: 1, FingerprintSHA256: strings.Repeat("c", 64), PublicKeyPEM: "-----BEGIN PUBLIC KEY-----\nTEST\n-----END PUBLIC KEY-----", KeyBits: 3072, Algorithm: "RSA-OAEP-SHA256", Status: "ACTIVE", CreatedAt: time.Now()}
}

// newEncryptionContractRouter 创建已注入可信认证和租户上下文的测试路由。
func newEncryptionContractRouter(register func(*gin.Engine)) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(middleware.ContextUserID, uint64(7))
		c.Set(middleware.ContextTenantID, uint64(3))
		c.Next()
	})
	register(router)
	return router
}
