package service

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
)

// encryptionRepositoryStub 是服务测试共享的内存聚合仓储，可注入状态冲突和完成失败。
type encryptionRepositoryStub struct {
	aggregate         repository.EncryptionTaskAggregate
	object            *domain.CiphertextObject
	findStagingObject *domain.CiphertextObject
	saveStagingError  error
	stagingAfterError *domain.CiphertextObject
	orphans           []domain.OrphanStorageObject
	progressError     error
	completeError     error
	lastCompletion    repository.EncryptionCompletion
	createCalls       int
	createAuditEvents []domain.AuditOutboxEvent
	receivedUserID    uint64
	protectedKey      *domain.ProtectedKey
	rsaBinding        *domain.RSAProtectedKeyBinding
	rsaPublicKey      *domain.RSAPublicKey
}

// ListAvailableAlgorithms 返回首期启用算法。
func (r *encryptionRepositoryStub) ListAvailableAlgorithms(context.Context, uint64) ([]domain.EncryptionAlgorithm, error) {
	return []domain.EncryptionAlgorithm{{Code: "RSA-OAEP-SHA256", Version: "1", AuthorizationType: "RSA_RECIPIENT", Status: "ACTIVE"}}, nil
}

// CreateTask 按幂等键返回或保存任务聚合。
func (r *encryptionRepositoryStub) CreateTask(_ context.Context, file domain.EncryptedFile, task domain.EncryptionTask, attempt domain.EncryptionTaskAttempt, auditEvents ...domain.AuditOutboxEvent) (repository.EncryptionTaskAggregate, bool, error) {
	r.createCalls++
	r.createAuditEvents = append([]domain.AuditOutboxEvent(nil), auditEvents...)
	if r.aggregate.Task.PublicID != "" {
		return r.aggregate, true, nil
	}
	file.ID, task.ID, attempt.ID = 11, 12, 13
	task.FileID, file.CurrentTaskID, attempt.TaskID = file.ID, task.ID, task.ID
	r.aggregate = repository.EncryptionTaskAggregate{File: file, Task: task, Attempt: attempt}
	return r.aggregate, false, nil
}

// FindTask 按可信租户和所有者返回任务。
func (r *encryptionRepositoryStub) FindTask(_ context.Context, tenantID, ownerUserID uint64, taskPublicID string) (repository.EncryptionTaskAggregate, error) {
	if r.aggregate.Task.TenantID != tenantID || r.aggregate.Task.OwnerUserID != ownerUserID || r.aggregate.Task.PublicID != taskPublicID {
		return repository.EncryptionTaskAggregate{}, repository.ErrEncryptionTaskNotFound
	}
	return r.aggregate, nil
}

// FindAttempt 按任务与执行 UUID 返回当前执行。
func (r *encryptionRepositoryStub) FindAttempt(_ context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string) (domain.EncryptionTaskAttempt, error) {
	aggregate, err := r.FindTask(context.Background(), tenantID, ownerUserID, taskPublicID)
	if err != nil || aggregate.Attempt.PublicID != attemptPublicID {
		return domain.EncryptionTaskAttempt{}, repository.ErrEncryptionAttemptNotFound
	}
	return aggregate.Attempt, nil
}

// UpdateProgress 保存合法测试进度或返回注入错误。
func (r *encryptionRepositoryStub) UpdateProgress(_ context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string, status domain.EncryptionTaskStatus, processedBytes int64) (domain.EncryptionTaskAttempt, error) {
	if r.progressError != nil {
		return domain.EncryptionTaskAttempt{}, r.progressError
	}
	attempt, err := r.FindAttempt(context.Background(), tenantID, ownerUserID, taskPublicID, attemptPublicID)
	if err != nil {
		return attempt, err
	}
	if processedBytes < attempt.ProcessedBytes || processedBytes > attempt.TotalBytes {
		return attempt, repository.ErrEncryptionStateConflict
	}
	attempt.Status, attempt.ProcessedBytes = status, processedBytes
	r.aggregate.Attempt, r.aggregate.Task.Status = attempt, status
	return attempt, nil
}

// FindAttemptStagingObject 返回当前执行已登记的暂存对象，供上传恢复测试使用。
func (r *encryptionRepositoryStub) FindAttemptStagingObject(_ context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string) (*domain.CiphertextObject, error) {
	if _, err := r.FindAttempt(context.Background(), tenantID, ownerUserID, taskPublicID, attemptPublicID); err != nil {
		return nil, err
	}
	return r.findStagingObject, nil
}

// SaveStagingObject 保存暂存密文对象。
func (r *encryptionRepositoryStub) SaveStagingObject(_ context.Context, object domain.CiphertextObject) error {
	if r.saveStagingError != nil {
		r.findStagingObject = r.stagingAfterError
		return r.saveStagingError
	}
	object.ID = 14
	r.object = &object
	r.findStagingObject = &object
	return nil
}

// Complete 原子完成测试聚合或返回注入事务失败。
func (r *encryptionRepositoryStub) Complete(_ context.Context, input repository.EncryptionCompletion) (repository.EncryptionTaskAggregate, bool, error) {
	if r.completeError != nil {
		return r.aggregate, false, r.completeError
	}
	r.lastCompletion = input
	if r.aggregate.Task.Status == domain.EncryptionCompleted {
		return r.aggregate, true, nil
	}
	now := time.Now()
	r.aggregate.Task.Status, r.aggregate.Attempt.Status, r.aggregate.File.Status = domain.EncryptionCompleted, domain.EncryptionCompleted, domain.EncryptedFileAvailable
	r.aggregate.Task.CompletedAt, r.aggregate.File.CompletedAt = &now, &now
	if r.object != nil {
		r.object.ObjectKey, r.object.Status = input.Object.ObjectKey, domain.CiphertextAvailable
	}
	return r.aggregate, false, nil
}

// MarkTerminal 保存失败或取消终态。
func (r *encryptionRepositoryStub) MarkTerminal(_ context.Context, tenantID, ownerUserID uint64, taskPublicID, attemptPublicID string, status domain.EncryptionTaskStatus, failureCode string, retryable bool) error {
	if _, err := r.FindAttempt(context.Background(), tenantID, ownerUserID, taskPublicID, attemptPublicID); err != nil {
		return err
	}
	if r.aggregate.Task.Status == domain.EncryptionCompleted {
		return repository.ErrEncryptionStateConflict
	}
	r.aggregate.Task.Status, r.aggregate.Attempt.Status, r.aggregate.Task.FailureCode, r.aggregate.Task.Retryable = status, status, failureCode, retryable
	return nil
}

// CreateRetry 为可重试失败聚合创建递增执行。
func (r *encryptionRepositoryStub) CreateRetry(_ context.Context, tenantID, ownerUserID uint64, taskPublicID string, attempt domain.EncryptionTaskAttempt) (repository.EncryptionTaskAggregate, error) {
	if _, err := r.FindTask(context.Background(), tenantID, ownerUserID, taskPublicID); err != nil {
		return r.aggregate, err
	}
	if r.aggregate.Task.Status != domain.EncryptionFailed || !r.aggregate.Task.Retryable {
		return r.aggregate, repository.ErrEncryptionStateConflict
	}
	attempt.ID, attempt.TenantID, attempt.TaskID, attempt.AttemptNo, attempt.TotalBytes = 15, tenantID, r.aggregate.Task.ID, r.aggregate.Attempt.AttemptNo+1, r.aggregate.Attempt.TotalBytes
	r.aggregate.Attempt, r.aggregate.Task.Status, r.aggregate.Task.CurrentAttemptNo = attempt, domain.EncryptionPending, attempt.AttemptNo
	return r.aggregate, nil
}

// ListOwnedFiles 返回当前所有者单条分页结果。
func (r *encryptionRepositoryStub) ListOwnedFiles(_ context.Context, tenantID, ownerUserID uint64, status domain.EncryptedFileStatus, _, _ int) (repository.EncryptedFilePage, error) {
	if r.aggregate.File.TenantID != tenantID || r.aggregate.File.OwnerUserID != ownerUserID || status != "" && r.aggregate.File.Status != status {
		return repository.EncryptedFilePage{}, nil
	}
	return repository.EncryptedFilePage{Items: []repository.FileCenterItem{fileCenterTestItem(r.aggregate.File, ownerUserID)}, Total: 1}, nil
}

// ListTenantFiles 返回企业云盘测试文件元数据，不包含密钥材料。
func (r *encryptionRepositoryStub) ListTenantFiles(_ context.Context, tenantID, actorUserID uint64, status domain.EncryptedFileStatus, _, _ int) (repository.EncryptedFilePage, error) {
	if r.aggregate.File.TenantID != tenantID || status != "" && r.aggregate.File.Status != status {
		return repository.EncryptedFilePage{}, nil
	}
	return repository.EncryptedFilePage{Items: []repository.FileCenterItem{fileCenterTestItem(r.aggregate.File, actorUserID)}, Total: 1}, nil
}

// FindTenantFile 返回同租户 AVAILABLE 文件，模拟企业云盘可见成员与所有者共享密文读取边界。
func (r *encryptionRepositoryStub) FindTenantFile(_ context.Context, tenantID, actorUserID uint64, filePublicID string) (repository.EncryptedFileDetail, error) {
	if r.aggregate.File.TenantID != tenantID || r.aggregate.File.PublicID != filePublicID || r.aggregate.File.Status != domain.EncryptedFileAvailable {
		return repository.EncryptedFileDetail{}, repository.ErrEncryptedFileNotFound
	}
	return repository.EncryptedFileDetail{Summary: fileCenterTestItem(r.aggregate.File, actorUserID), File: r.aggregate.File, Task: r.aggregate.Task, Object: r.object}, nil
}

// FindOwnedFile 返回任务、对象和脱敏密钥元数据。
func (r *encryptionRepositoryStub) FindOwnedFile(_ context.Context, tenantID, ownerUserID uint64, filePublicID string) (repository.EncryptedFileDetail, error) {
	if r.aggregate.File.TenantID != tenantID || r.aggregate.File.OwnerUserID != ownerUserID || r.aggregate.File.PublicID != filePublicID {
		return repository.EncryptedFileDetail{}, repository.ErrEncryptedFileNotFound
	}
	return repository.EncryptedFileDetail{File: r.aggregate.File, Task: r.aggregate.Task, Object: r.object}, nil
}

// ListReceivedFiles 返回测试接收者绑定的单条文件。
func (r *encryptionRepositoryStub) ListReceivedFiles(_ context.Context, tenantID, recipientUserID uint64, _, _ int) (repository.EncryptedFilePage, error) {
	if r.aggregate.File.TenantID != tenantID || recipientUserID == 0 || r.receivedUserID != 0 && r.receivedUserID != recipientUserID {
		return repository.EncryptedFilePage{}, nil
	}
	return repository.EncryptedFilePage{Items: []repository.FileCenterItem{fileCenterTestItem(r.aggregate.File, recipientUserID)}, Total: 1}, nil
}

// fileCenterTestItem 构造服务层测试用的密文列表摘要，不模拟解密授权状态。
func fileCenterTestItem(file domain.EncryptedFile, actorUserID uint64) repository.FileCenterItem {
	return repository.FileCenterItem{ID: file.PublicID, OriginalFilename: file.OriginalFilename, PlaintextSize: file.PlaintextSize, Status: file.Status, OwnerUserID: file.OwnerUserID, Owner: repository.FileCenterUserSummary{UserID: file.OwnerUserID}, CreatedAt: file.CreatedAt, CompletedAt: file.CompletedAt}
}

// FindReceivedFile 返回当前可见文件的详情；测试替身不使用接收者身份作为解密门槛。
func (r *encryptionRepositoryStub) FindReceivedFile(_ context.Context, tenantID, recipientUserID uint64, filePublicID string) (repository.EncryptedFileDetail, error) {
	if r.aggregate.File.TenantID != tenantID || recipientUserID == 0 || r.aggregate.File.PublicID != filePublicID {
		return repository.EncryptedFileDetail{}, repository.ErrEncryptedFileNotFound
	}
	return repository.EncryptedFileDetail{File: r.aggregate.File, Task: r.aggregate.Task, Object: r.object, ProtectedKey: r.protectedKey, RSABinding: r.rsaBinding, RSAPublicKey: r.rsaPublicKey}, nil
}

// RegisterOrphan 记录完成失败后的补偿对象。
func (r *encryptionRepositoryStub) RegisterOrphan(_ context.Context, orphan domain.OrphanStorageObject) error {
	r.orphans = append(r.orphans, orphan)
	return nil
}

// ClaimOrphans 返回待清理孤儿对象。
func (r *encryptionRepositoryStub) ClaimOrphans(context.Context, int, time.Time) ([]domain.OrphanStorageObject, error) {
	return append([]domain.OrphanStorageObject(nil), r.orphans...), nil
}

// SaveOrphanResult 更新测试孤儿对象结果。
func (r *encryptionRepositoryStub) SaveOrphanResult(_ context.Context, orphan domain.OrphanStorageObject) error {
	for index := range r.orphans {
		if r.orphans[index].ObjectKey == orphan.ObjectKey {
			r.orphans[index] = orphan
		}
	}
	return nil
}

// encryptedStorageStub 是服务测试的内存密文存储，可注入提交和删除失败。
type encryptedStorageStub struct {
	bytes       []byte
	objectKey   string
	commitError error
	openError   error
	deleteError error
	deleted     int
	stageCalls  int
}

// StageCiphertext 保存受限上传并复核调用方摘要。
func (s *encryptedStorageStub) StageCiphertext(_ context.Context, _ uint64, _ string, reader io.Reader, _ int64, expected string) (storage.CiphertextUploadResult, error) {
	s.stageCalls++
	value, err := io.ReadAll(reader)
	if err != nil {
		return storage.CiphertextUploadResult{}, err
	}
	s.bytes, s.objectKey = value, ".staging/object.part"
	return storage.CiphertextUploadResult{ObjectKey: s.objectKey, Size: int64(len(value)), SHA256: expected}, nil
}

// CommitCiphertext 返回正式对象键或注入失败。
func (s *encryptedStorageStub) CommitCiphertext(context.Context, string) (string, error) {
	if s.commitError != nil {
		return "", s.commitError
	}
	s.objectKey = "1/object.cipher"
	return s.objectKey, nil
}

// OpenCiphertext 返回内存密文流。
func (s *encryptedStorageStub) OpenCiphertext(context.Context, string) (io.ReadCloser, error) {
	if s.openError != nil {
		return nil, s.openError
	}
	return io.NopCloser(bytes.NewReader(s.bytes)), nil
}

// DeleteCiphertext 幂等记录删除或注入失败。
func (s *encryptedStorageStub) DeleteCiphertext(context.Context, string) error {
	if s.deleteError != nil {
		return s.deleteError
	}
	s.deleted++
	return nil
}

// DeleteExpiredStaging 返回无过期对象。
func (s *encryptedStorageStub) DeleteExpiredStaging(context.Context, time.Time, int) (int, error) {
	return 0, nil
}

// auditRecorderStub 收集脱敏审计事件供安全路径断言。
type auditRecorderStub struct {
	events []AuditEvent
	err    error
}

// Record 保存审计事件或返回注入错误。
func (r *auditRecorderStub) Record(_ context.Context, event AuditEvent) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, event)
	return nil
}

// newEncryptionServiceFixture 创建算法、公钥、Redis 准入、存储和审计均可观察的服务夹具。
func newEncryptionServiceFixture(redisClient *redis.Client) (*EncryptionService, *encryptionRepositoryStub, *encryptedStorageStub, *auditRecorderStub) {
	repositoryLayer := &encryptionRepositoryStub{}
	keyRepository := &rsaKeyRepositoryStub{keys: []domain.RSAPublicKey{
		{ID: 20, PublicID: "owner-key", TenantID: 3, UserID: 7, Version: 1, FingerprintSHA256: strings.Repeat("b", 64), PublicKeyPEM: "OWNER_PUBLIC", KeyBits: 3072, Algorithm: "RSA-OAEP-SHA256", Status: "ACTIVE"},
		{ID: 21, PublicID: "623e4567-e89b-42d3-a456-426614174000", TenantID: 3, UserID: 9, Version: 1, FingerprintSHA256: strings.Repeat("c", 64), PublicKeyPEM: "PUBLIC", KeyBits: 3072, Algorithm: "RSA-OAEP-SHA256", Status: "ACTIVE"},
	}}
	audit := &auditRecorderStub{}
	encryptedStorage := &encryptedStorageStub{}
	admission := NewEncryptionAdmission(redisClient, 3, time.Minute)
	return NewEncryptionService(repositoryLayer, NewRSAKeyService(keyRepository, audit), encryptedStorage, admission, audit, 1024), repositoryLayer, encryptedStorage, audit
}
