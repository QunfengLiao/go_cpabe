package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	cryptomodule "go-cpabe/backend/internal/crypto"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/identifier"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
)

// CreateEncryptionTaskInput 描述远程服务可接收的非明文文件元数据与算法授权选择。
type CreateEncryptionTaskInput struct {
	File struct {
		Name            string `json:"name"`
		Size            int64  `json:"size"`
		DisplayMIMEType string `json:"display_mime_type"`
	} `json:"file"`
	Algorithm struct {
		Code    string `json:"code"`
		Version string `json:"version"`
	} `json:"algorithm"`
	Authorization map[string]any `json:"authorization"`
}

// ProgressInput 是本地执行器上报的真实阶段和字节数。
type ProgressInput struct {
	Stage          domain.EncryptionTaskStatus `json:"stage"`
	ProcessedBytes int64                       `json:"processed_bytes"`
	TotalBytes     int64                       `json:"total_bytes"`
}

// CompleteEncryptionInput 是客户端完成协议，不包含明文 DEK 或文件明文。
type CompleteEncryptionInput struct {
	UploadID          string `json:"upload_id"`
	ContentEncryption struct {
		Algorithm         string `json:"algorithm"`
		ContainerFormat   string `json:"container_format"`
		EncryptionVersion string `json:"encryption_version"`
		NoncePrefixBase64 string `json:"nonce_prefix_base64"`
		ChunkSize         uint32 `json:"chunk_size"`
		ChunkCount        uint32 `json:"chunk_count"`
		TagLength         int    `json:"tag_length"`
		AADVersion        string `json:"aad_version"`
		ContextSHA256     string `json:"context_sha256"`
	} `json:"content_encryption"`
	ProtectedKey struct {
		AlgorithmCode    string `json:"algorithm_code"`
		AlgorithmVersion string `json:"algorithm_version"`
		Format           string `json:"format"`
		ValueBase64      string `json:"value_base64"`
		ContextSHA256    string `json:"context_sha256"`
	} `json:"protected_key"`
	ProtectedKeys  []CompleteProtectedKeyInput `json:"protected_keys"`
	AdapterBinding map[string]any              `json:"adapter_binding"`
	Benchmark      struct {
		PlaintextSize            int64  `json:"plaintext_size"`
		CiphertextSize           int64  `json:"ciphertext_size"`
		AESEncryptMS             int64  `json:"aes_encrypt_ms"`
		DEKProtectMS             int64  `json:"dek_protect_ms"`
		UploadMS                 int64  `json:"upload_ms"`
		ValidationDurationMS     int64  `json:"validation_duration_ms"`
		FileEncryptionDurationMS int64  `json:"file_encryption_duration_ms"`
		KeyProtectionDurationMS  int64  `json:"key_protection_duration_ms"`
		UploadDurationMS         int64  `json:"upload_duration_ms"`
		MetadataCommitDurationMS int64  `json:"metadata_commit_duration_ms"`
		TotalDurationMS          int64  `json:"total_duration_ms"`
		PlaintextSizeBytes       int64  `json:"plaintext_size_bytes"`
		CiphertextSizeBytes      int64  `json:"ciphertext_size_bytes"`
		PeakWorkingSetBytes      int64  `json:"peak_working_set_bytes"`
		ClientRuntime            string `json:"client_runtime"`
	} `json:"benchmark"`
}

// CompleteProtectedKeyInput 表示一个接收者提交的一份受保护 DEK 和算法专属绑定。
type CompleteProtectedKeyInput struct {
	AlgorithmCode    string         `json:"algorithm_code"`
	AlgorithmVersion string         `json:"algorithm_version"`
	Format           string         `json:"format"`
	ValueBase64      string         `json:"value_base64"`
	ContextSHA256    string         `json:"context_sha256"`
	AdapterBinding   map[string]any `json:"adapter_binding"`
}

// EncryptionService 编排远程任务、密文暂存和元数据完成；密码学计算全部留在本地 Worker。
type EncryptionService struct {
	repository repository.EncryptionRepository
	storage    storage.EncryptedFileStorage
	validator  *cryptomodule.MetadataValidator
	adapters   map[string]EncryptionAlgorithmAdapter
	admission  *EncryptionAdmission
	audit      AuditRecorder
	maxSize    int64
}

// NewEncryptionService 创建加密编排服务。
func NewEncryptionService(repository repository.EncryptionRepository, keys *RSAKeyService, encryptedStorage storage.EncryptedFileStorage, admission *EncryptionAdmission, audit AuditRecorder, maxSize int64) *EncryptionService {
	rsaAdapter := NewRSAEncryptionAlgorithmAdapter(keys)
	return &EncryptionService{repository: repository, storage: encryptedStorage, validator: cryptomodule.NewMetadataValidator(), adapters: map[string]EncryptionAlgorithmAdapter{adapterKey(rsaAdapter.Code(), rsaAdapter.Version()): rsaAdapter}, admission: admission, audit: audit, maxSize: maxSize}
}

// Algorithms 返回当前租户真实启用的算法目录，不生成伪 CP-ABE 占位项。
func (s *EncryptionService) Algorithms(ctx context.Context, tenantID uint64) ([]domain.EncryptionAlgorithm, error) {
	algorithms, err := s.repository.ListAvailableAlgorithms(ctx, tenantID)
	if err != nil {
		return nil, response.ErrInternal
	}
	return algorithms, nil
}

// CreateTask 校验算法、公钥和不可变授权快照后创建幂等任务并领取并发租约。
func (s *EncryptionService) CreateTask(ctx context.Context, tenantID, actorUserID uint64, idempotencyKey string, input CreateEncryptionTaskInput) (repository.EncryptionTaskAggregate, bool, error) {
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return repository.EncryptionTaskAggregate{}, false, response.ErrBadRequest
	}
	name := filepath.Base(strings.TrimSpace(input.File.Name))
	if name == "." || name == "" || len([]rune(name)) > 255 || input.File.Size <= 0 || input.File.Size > s.maxSize {
		return repository.EncryptionTaskAggregate{}, false, response.ErrEncryptionFileInvalid
	}
	algorithms, err := s.repository.ListAvailableAlgorithms(ctx, tenantID)
	if err != nil {
		return repository.EncryptionTaskAggregate{}, false, response.ErrInternal
	}
	if !algorithmEnabled(algorithms, input.Algorithm.Code, input.Algorithm.Version) {
		return repository.EncryptionTaskAggregate{}, false, response.ErrEncryptionAlgorithmUnavailable
	}
	adapter, ok := s.adapters[adapterKey(input.Algorithm.Code, input.Algorithm.Version)]
	if !ok {
		return repository.EncryptionTaskAggregate{}, false, response.ErrEncryptionAlgorithmUnavailable
	}
	snapshot, err := adapter.ValidateAuthorization(ctx, tenantID, input.Authorization)
	if err != nil {
		return repository.EncryptionTaskAggregate{}, false, err
	}
	if !authorizationIncludesOwner(snapshot, actorUserID) {
		return repository.EncryptionTaskAggregate{}, false, response.ErrEncryptionOwnerKeyRequired
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		return repository.EncryptionTaskAggregate{}, false, response.ErrInternal
	}
	snapshotHash := sha256.Sum256(snapshotBytes)
	taskID, err := identifier.NewUUID()
	if err != nil {
		return repository.EncryptionTaskAggregate{}, false, response.ErrInternal
	}
	fileID, err := identifier.NewUUID()
	if err != nil {
		return repository.EncryptionTaskAggregate{}, false, response.ErrInternal
	}
	attemptID, err := identifier.NewUUID()
	if err != nil {
		return repository.EncryptionTaskAggregate{}, false, response.ErrInternal
	}
	if err := s.admission.Acquire(ctx, tenantID, actorUserID, attemptID); err != nil {
		return repository.EncryptionTaskAggregate{}, false, err
	}
	// StartedAt/UpdatedAt 不是 Gorm 约定字段，服务层显式冻结执行起点，避免 MySQL 严格模式拒绝零时间。
	now := time.Now()
	file := domain.EncryptedFile{PublicID: fileID, TenantID: tenantID, OwnerUserID: actorUserID, OriginalFilename: name, DisplayMIMEType: strings.TrimSpace(input.File.DisplayMIMEType), PlaintextSize: input.File.Size, Status: domain.EncryptedFileDraft}
	task := domain.EncryptionTask{PublicID: taskID, TenantID: tenantID, OwnerUserID: actorUserID, IdempotencyKey: idempotencyKey, AlgorithmCode: input.Algorithm.Code, AlgorithmVersion: input.Algorithm.Version, AuthorizationType: stringValue(snapshot["type"]), AuthorizationSnapshot: snapshotBytes, AuthorizationSnapshotSHA256: hex.EncodeToString(snapshotHash[:]), Status: domain.EncryptionPending, CurrentAttemptNo: 1, LockVersion: 1}
	attempt := domain.EncryptionTaskAttempt{PublicID: attemptID, TenantID: tenantID, AttemptNo: 1, Status: domain.EncryptionPending, TotalBytes: input.File.Size, StartedAt: now, UpdatedAt: now}
	authorizationAudit := map[string]any{"task_id": taskID, "algorithm_code": task.AlgorithmCode, "algorithm_version": task.AlgorithmVersion}
	copyAllowedAuthorizationAudit(authorizationAudit, snapshot)
	createAuditEvents := []AuditEvent{
		{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.task.create", TargetType: "encryption", TargetPublicID: taskID, Result: "SUCCESS", SourceTrust: "SERVER_OBSERVED", DedupKey: auditDedupKey(tenantID, taskID, attemptID, "encryption.task.create"), Metadata: map[string]any{"task_id": taskID, "attempt_id": attemptID, "algorithm_code": task.AlgorithmCode, "algorithm_version": task.AlgorithmVersion}},
		{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.authorization.validated", TargetType: "encryption", TargetPublicID: taskID, Result: "SUCCESS", SourceTrust: "SERVER_OBSERVED", DedupKey: auditDedupKey(tenantID, taskID, attemptID, "encryption.authorization.validated"), Metadata: authorizationAudit},
	}
	preparedAudit, atomicAudit, err := prepareTransactionalAuditEvents(s.audit, createAuditEvents)
	if err != nil {
		s.admission.Release(ctx, tenantID, attemptID)
		return repository.EncryptionTaskAggregate{}, false, response.ErrInternal
	}
	aggregate, idempotent, err := s.repository.CreateTask(ctx, file, task, attempt, preparedAudit...)
	if err != nil {
		s.admission.Release(ctx, tenantID, attemptID)
		return repository.EncryptionTaskAggregate{}, false, response.ErrInternal
	}
	if idempotent {
		s.admission.Release(ctx, tenantID, attemptID)
		if aggregate.Task.AuthorizationSnapshotSHA256 != task.AuthorizationSnapshotSHA256 || aggregate.Task.AlgorithmCode != task.AlgorithmCode || aggregate.Task.AlgorithmVersion != task.AlgorithmVersion || aggregate.File.PlaintextSize != file.PlaintextSize || aggregate.File.OriginalFilename != file.OriginalFilename {
			return repository.EncryptionTaskAggregate{}, false, response.ErrEncryptionStateConflict
		}
	}
	if !atomicAudit && !idempotent {
		for _, event := range createAuditEvents {
			recordAuditBestEffort(ctx, s.audit, event)
		}
	}
	return aggregate, idempotent, nil
}

// Task 返回当前 DO 自己的任务和当前执行。
func (s *EncryptionService) Task(ctx context.Context, tenantID, actorUserID uint64, taskPublicID string) (repository.EncryptionTaskAggregate, error) {
	result, err := s.repository.FindTask(ctx, tenantID, actorUserID, taskPublicID)
	return result, mapEncryptionRepositoryError(err)
}

// ReportProgress 验证总字节不变并按合法状态图更新真实进度。
func (s *EncryptionService) ReportProgress(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID string, input ProgressInput) (repository.EncryptionTaskAggregate, error) {
	attempt, err := s.repository.FindAttempt(ctx, tenantID, actorUserID, taskPublicID, attemptPublicID)
	if err != nil {
		return repository.EncryptionTaskAggregate{}, mapEncryptionRepositoryError(err)
	}
	if input.TotalBytes != attempt.TotalBytes {
		return repository.EncryptionTaskAggregate{}, response.ErrEncryptionStateConflict
	}
	if _, err := s.repository.UpdateProgress(ctx, tenantID, actorUserID, taskPublicID, attemptPublicID, input.Stage, input.ProcessedBytes); err != nil {
		return repository.EncryptionTaskAggregate{}, mapEncryptionRepositoryError(err)
	}
	recordAuditBestEffort(ctx, s.audit, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.progress", TargetType: "encryption", TargetPublicID: attemptPublicID, Result: "SUCCESS", SourceTrust: "CLIENT_REPORTED", Metadata: map[string]any{"task_id": taskPublicID, "attempt_id": attemptPublicID, "stage": string(input.Stage), "processed_bytes": input.ProcessedBytes}})
	return s.Task(ctx, tenantID, actorUserID, taskPublicID)
}

// UploadCiphertext 在服务端流式复核大小和摘要，成功对象仍处于不可下载的 STAGING 状态。
func (s *EncryptionService) UploadCiphertext(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID, expectedSHA256, format string, contentLength int64, reader io.Reader) (domain.CiphertextObject, error) {
	if format != cryptomodule.ContainerMagic || contentLength <= 0 || contentLength > s.maxSize+s.maxSize/100+16*1024 {
		return domain.CiphertextObject{}, response.ErrEncryptionFileTooLarge
	}
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	digest, err := hex.DecodeString(expectedSHA256)
	if err != nil || len(digest) != sha256.Size {
		return domain.CiphertextObject{}, response.ErrCiphertextHashMismatch
	}
	if _, err := s.repository.FindAttempt(ctx, tenantID, actorUserID, taskPublicID, attemptPublicID); err != nil {
		return domain.CiphertextObject{}, mapEncryptionRepositoryError(err)
	}
	if existing, err := s.repository.FindAttemptStagingObject(ctx, tenantID, actorUserID, taskPublicID, attemptPublicID); err != nil {
		return domain.CiphertextObject{}, mapEncryptionRepositoryError(err)
	} else if existing != nil {
		if !sameStagingUpload(*existing, format, expectedSHA256, contentLength) {
			return domain.CiphertextObject{}, response.ErrEncryptionStateConflict
		}
		recordAuditBestEffort(ctx, s.audit, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.ciphertext.upload", TargetType: "encryption", TargetPublicID: existing.PublicID, Result: "SUCCESS", SourceTrust: "SERVER_OBSERVED", Metadata: map[string]any{"task_id": taskPublicID, "attempt_id": attemptPublicID, "ciphertext_size": existing.CiphertextSize, "ciphertext_sha256": existing.CiphertextSHA256}})
		return *existing, nil
	}
	result, err := s.storage.StageCiphertext(ctx, tenantID, attemptPublicID, reader, s.maxSize+s.maxSize/100+16*1024, expectedSHA256)
	if err != nil {
		code := response.ErrCiphertextStorageFailed
		if strings.Contains(strings.ToLower(err.Error()), "hash") {
			code = response.ErrCiphertextHashMismatch
		}
		_ = s.record(ctx, tenantID, actorUserID, "encryption.ciphertext.upload", attemptPublicID, "FAILURE", "SERVER_OBSERVED", code.Code, map[string]any{"task_id": taskPublicID, "attempt_id": attemptPublicID})
		return domain.CiphertextObject{}, code
	}
	if contentLength != result.Size {
		_ = s.storage.DeleteCiphertext(ctx, result.ObjectKey)
		return domain.CiphertextObject{}, response.ErrCiphertextHashMismatch
	}
	uploadID, err := identifier.NewUUID()
	if err != nil {
		_ = s.storage.DeleteCiphertext(ctx, result.ObjectKey)
		return domain.CiphertextObject{}, response.ErrInternal
	}
	attempt, err := s.repository.FindAttempt(ctx, tenantID, actorUserID, taskPublicID, attemptPublicID)
	if err != nil {
		_ = s.storage.DeleteCiphertext(ctx, result.ObjectKey)
		return domain.CiphertextObject{}, mapEncryptionRepositoryError(err)
	}
	object := domain.CiphertextObject{PublicID: uploadID, TenantID: tenantID, TaskAttemptID: attempt.ID, ObjectKey: result.ObjectKey, StorageBackend: "LOCAL", ContainerFormat: format, CiphertextSize: result.Size, CiphertextSHA256: result.SHA256, Status: domain.CiphertextStaging}
	if err := s.repository.SaveStagingObject(ctx, object); err != nil {
		_ = s.storage.DeleteCiphertext(ctx, result.ObjectKey)
		// 唯一键冲突通常表示并发重试已有请求先登记成功；回查并比较内容事实后复用胜出对象。
		existing, findErr := s.repository.FindAttemptStagingObject(ctx, tenantID, actorUserID, taskPublicID, attemptPublicID)
		if findErr == nil && existing != nil && sameStagingUpload(*existing, format, expectedSHA256, contentLength) {
			return *existing, nil
		}
		if findErr == nil && existing != nil {
			return domain.CiphertextObject{}, response.ErrEncryptionStateConflict
		}
		return domain.CiphertextObject{}, response.ErrInternal
	}
	recordAuditBestEffort(ctx, s.audit, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.ciphertext.upload", TargetType: "encryption", TargetPublicID: uploadID, Result: "SUCCESS", SourceTrust: "SERVER_OBSERVED", Metadata: map[string]any{"task_id": taskPublicID, "attempt_id": attemptPublicID, "ciphertext_size": result.Size, "ciphertext_sha256": result.SHA256}})
	return object, nil
}

// Complete 校验算法无关受保护密钥和 RSA 专属绑定，提交对象后原子完成数据库事实。
func (s *EncryptionService) Complete(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID string, input CompleteEncryptionInput) (repository.EncryptionTaskAggregate, bool, error) {
	aggregate, err := s.repository.FindTask(ctx, tenantID, actorUserID, taskPublicID)
	if err != nil {
		return aggregate, false, mapEncryptionRepositoryError(err)
	}
	if aggregate.Task.Status == domain.EncryptionCompleted {
		return aggregate, true, nil
	}
	// 兼容迁移前已完成的客户端请求；新客户端始终显式提交版本，服务端仍只接受当前容器版本。
	if input.ContentEncryption.EncryptionVersion == "" {
		input.ContentEncryption.EncryptionVersion = "1"
	}
	if input.ContentEncryption.Algorithm != "AES-256-GCM" || input.ContentEncryption.ContainerFormat != cryptomodule.ContainerMagic || input.ContentEncryption.EncryptionVersion != "1" || input.ContentEncryption.ChunkSize != cryptomodule.DefaultChunkSize || input.ContentEncryption.TagLength != cryptomodule.GCMTagSize || input.ContentEncryption.AADVersion != "1" || input.ContentEncryption.NoncePrefixBase64 == "" {
		return aggregate, false, response.ErrProtectedKeyInvalid
	}
	if nonce, decodeErr := base64.StdEncoding.DecodeString(input.ContentEncryption.NoncePrefixBase64); decodeErr != nil || len(nonce) != 8 {
		return aggregate, false, response.ErrProtectedKeyInvalid
	}
	adapter, ok := s.adapters[adapterKey(input.ProtectedKey.AlgorithmCode, input.ProtectedKey.AlgorithmVersion)]
	if len(input.ProtectedKeys) > 0 {
		adapter, ok = s.adapters[adapterKey(input.ProtectedKeys[0].AlgorithmCode, input.ProtectedKeys[0].AlgorithmVersion)]
	}
	if !ok {
		return aggregate, false, response.ErrEncryptionAlgorithmUnavailable
	}
	var authorizationSnapshot map[string]any
	if err := json.Unmarshal(aggregate.Task.AuthorizationSnapshot, &authorizationSnapshot); err != nil {
		return aggregate, false, response.ErrInternal
	}
	protectedInputs := input.ProtectedKeys
	if len(protectedInputs) == 0 {
		protectedInputs = []CompleteProtectedKeyInput{{AlgorithmCode: input.ProtectedKey.AlgorithmCode, AlgorithmVersion: input.ProtectedKey.AlgorithmVersion, Format: input.ProtectedKey.Format, ValueBase64: input.ProtectedKey.ValueBase64, ContextSHA256: input.ProtectedKey.ContextSHA256, AdapterBinding: input.AdapterBinding}}
	}
	completedKeys := make([]repository.ProtectedKeyCompletion, 0, len(protectedInputs))
	expectedBindings := frozenRecipientBindings(authorizationSnapshot)
	seenBindings := make(map[string]struct{}, len(protectedInputs))
	if len(expectedBindings) > 0 && len(protectedInputs) != len(expectedBindings) {
		return aggregate, false, response.ErrProtectedKeyInvalid
	}
	protectedKeyTotalSize := int64(0)
	for _, protectedInput := range protectedInputs {
		if len(expectedBindings) > 0 {
			identity, ok := submittedRecipientBinding(protectedInput.AdapterBinding)
			if !ok {
				return aggregate, false, response.ErrProtectedKeyInvalid
			}
			if _, expected := expectedBindings[identity]; !expected {
				return aggregate, false, response.ErrProtectedKeyInvalid
			}
			if _, duplicate := seenBindings[identity]; duplicate {
				return aggregate, false, response.ErrProtectedKeyInvalid
			}
			seenBindings[identity] = struct{}{}
		}
		protectedBytes, err := base64.StdEncoding.DecodeString(protectedInput.ValueBase64)
		if err != nil {
			return aggregate, false, response.ErrProtectedKeyInvalid
		}
		protectedKeyTotalSize += int64(len(protectedBytes))
		if protectedInput.AlgorithmCode != aggregate.Task.AlgorithmCode || protectedInput.AlgorithmVersion != aggregate.Task.AlgorithmVersion {
			zeroSensitiveBytes(protectedBytes)
			return aggregate, false, response.ErrProtectedKeyInvalid
		}
		result := cryptomodule.ProtectedKeyResult{AlgorithmCode: protectedInput.AlgorithmCode, AlgorithmVersion: protectedInput.AlgorithmVersion, Format: protectedInput.Format, Value: protectedBytes, ContextSHA256: protectedInput.ContextSHA256}
		result, bindingPlan, err := adapter.ValidateCompletion(ctx, tenantID, authorizationSnapshot, result, protectedInput.AdapterBinding)
		if err != nil {
			zeroSensitiveBytes(protectedBytes)
			return aggregate, false, err
		}
		if err := s.validator.ValidateProtectedKey(result); err != nil || protectedInput.ContextSHA256 != input.ContentEncryption.ContextSHA256 {
			zeroSensitiveBytes(protectedBytes)
			return aggregate, false, response.ErrProtectedKeyInvalid
		}
		protectedID, err := identifier.NewUUID()
		if err != nil {
			zeroSensitiveBytes(protectedBytes)
			return aggregate, false, response.ErrInternal
		}
		completedKeys = append(completedKeys, repository.ProtectedKeyCompletion{ProtectedKey: domain.ProtectedKey{PublicID: protectedID, AlgorithmCode: protectedInput.AlgorithmCode, AlgorithmVersion: protectedInput.AlgorithmVersion, ProtectedKeyFormat: protectedInput.Format, ProtectedKeyBytes: append([]byte(nil), protectedBytes...), ContextSHA256: protectedInput.ContextSHA256}, AdapterBinding: bindingPlan})
		zeroSensitiveBytes(protectedBytes)
	}
	if len(completedKeys) == 0 {
		return aggregate, false, response.ErrProtectedKeyInvalid
	}
	if len(expectedBindings) > 0 && len(seenBindings) != len(expectedBindings) {
		return aggregate, false, response.ErrProtectedKeyInvalid
	}
	plaintextSize := firstPositive(input.Benchmark.PlaintextSizeBytes, input.Benchmark.PlaintextSize)
	ciphertextSize := firstPositive(input.Benchmark.CiphertextSizeBytes, input.Benchmark.CiphertextSize)
	aesEncryptMS := firstPositiveOrZero(input.Benchmark.FileEncryptionDurationMS, input.Benchmark.AESEncryptMS)
	keyProtectionMS := firstPositiveOrZero(input.Benchmark.KeyProtectionDurationMS, input.Benchmark.DEKProtectMS)
	uploadMS := firstPositiveOrZero(input.Benchmark.UploadDurationMS, input.Benchmark.UploadMS)
	if plaintextSize != aggregate.File.PlaintextSize || ciphertextSize <= 0 || input.Benchmark.ClientRuntime != "LOCAL_GO_WORKER" {
		return aggregate, false, response.ErrBadRequest
	}
	completionAuditEvents := []AuditEvent{
		{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.aes.complete", TargetType: "encryption", TargetPublicID: attemptPublicID, Result: "SUCCESS", SourceTrust: "CLIENT_REPORTED", DedupKey: auditDedupKey(tenantID, taskPublicID, attemptPublicID, "encryption.aes.complete"), Metadata: map[string]any{"task_id": taskPublicID, "attempt_id": attemptPublicID, "stage": string(domain.EncryptionEncryptingFile), "plaintext_size": plaintextSize, "ciphertext_size": ciphertextSize, "aes_encrypt_ms": aesEncryptMS}},
		{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.dek.protect", TargetType: "encryption", TargetPublicID: attemptPublicID, Result: "SUCCESS", SourceTrust: "CLIENT_REPORTED", DedupKey: auditDedupKey(tenantID, taskPublicID, attemptPublicID, "encryption.dek.protect"), Metadata: map[string]any{"task_id": taskPublicID, "attempt_id": attemptPublicID, "algorithm_code": aggregate.Task.AlgorithmCode, "algorithm_version": aggregate.Task.AlgorithmVersion, "dek_protect_ms": keyProtectionMS, "recipient_count": len(completedKeys), "protected_key_total_size": protectedKeyTotalSize}},
		{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.complete", TargetType: "encryption", TargetPublicID: aggregate.File.PublicID, Result: "SUCCESS", SourceTrust: "SERVER_OBSERVED", DedupKey: auditDedupKey(tenantID, taskPublicID, attemptPublicID, "encryption.complete"), Metadata: map[string]any{"file_id": aggregate.File.PublicID, "task_id": taskPublicID, "attempt_id": attemptPublicID, "ciphertext_size": ciphertextSize, "algorithm_code": aggregate.Task.AlgorithmCode}},
	}
	preparedAudit, atomicAudit, err := prepareTransactionalAuditEvents(s.audit, completionAuditEvents)
	if err != nil {
		return aggregate, false, response.ErrInternal
	}
	detail, findErr := s.repository.FindOwnedFile(ctx, tenantID, actorUserID, aggregate.File.PublicID)
	if findErr != nil || detail.Object == nil || detail.Object.PublicID != input.UploadID {
		return aggregate, false, response.ErrCiphertextUploadRequired
	}
	finalKey, err := s.storage.CommitCiphertext(ctx, detail.Object.ObjectKey)
	if err != nil {
		return aggregate, false, response.ErrCiphertextStorageFailed
	}
	completion := repository.EncryptionCompletion{TenantID: tenantID, OwnerUserID: actorUserID, TaskPublicID: taskPublicID, AttemptPublicID: attemptPublicID, Object: *detail.Object,
		ProtectedKeys: completedKeys,
		ProtectedKey:  completedKeys[0].ProtectedKey,
		Benchmark: domain.EncryptionBenchmark{
			ValidationDurationMS: input.Benchmark.ValidationDurationMS, PlaintextSize: plaintextSize, CiphertextSize: ciphertextSize, ProtectedKeyTotalSizeBytes: protectedKeyTotalSize,
			AESEncryptMS: aesEncryptMS, DEKProtectMS: keyProtectionMS, KeyProtectionDurationMS: keyProtectionMS,
			UploadMS: uploadMS, MetadataCommitDurationMS: input.Benchmark.MetadataCommitDurationMS, TotalDurationMS: input.Benchmark.TotalDurationMS,
			RecipientCount: int64(len(completedKeys)), PeakWorkingSetBytes: input.Benchmark.PeakWorkingSetBytes,
			ClientRuntime: input.Benchmark.ClientRuntime, AlgorithmCode: aggregate.Task.AlgorithmCode, AlgorithmVersion: aggregate.Task.AlgorithmVersion, Result: "SUCCESS",
		}, AuditEvents: preparedAudit}
	completion.Object.ObjectKey = finalKey
	completed, idempotent, err := s.repository.Complete(ctx, completion)
	if err != nil {
		orphan := domain.OrphanStorageObject{TenantID: tenantID, TaskAttemptID: &aggregate.Attempt.ID, ObjectKey: finalKey, ReasonCode: "COMPLETE_TRANSACTION_FAILED", Status: "PENDING"}
		_ = s.repository.RegisterOrphan(ctx, orphan)
		return aggregate, false, mapEncryptionRepositoryError(err)
	}
	completion.Object.ContentAlgorithm = input.ContentEncryption.Algorithm
	completion.Object.EncryptionVersion = input.ContentEncryption.EncryptionVersion
	completion.Object.NoncePrefixBase64 = input.ContentEncryption.NoncePrefixBase64
	completion.Object.AuthenticationTagLength = input.ContentEncryption.TagLength
	completion.Object.AADVersion = input.ContentEncryption.AADVersion
	s.admission.Release(ctx, tenantID, attemptPublicID)
	if !atomicAudit && !idempotent {
		for _, event := range completionAuditEvents {
			recordAuditBestEffort(ctx, s.audit, event)
		}
	}
	return completed, idempotent, nil
}

// prepareTransactionalAuditEvents 仅对数据库审计实现生成同事务 outbox 行；替代实现仍在业务成功后使用原有尽力记录语义。
func prepareTransactionalAuditEvents(recorder AuditRecorder, events []AuditEvent) ([]domain.AuditOutboxEvent, bool, error) {
	databaseRecorder, ok := recorder.(*DatabaseAuditRecorder)
	if !ok || databaseRecorder == nil {
		return nil, false, nil
	}
	prepared := make([]domain.AuditOutboxEvent, 0, len(events))
	for _, event := range events {
		outboxEvent, err := databaseRecorder.Prepare(event)
		if err != nil {
			return nil, true, err
		}
		prepared = append(prepared, outboxEvent)
	}
	return prepared, true, nil
}

// auditDedupKey 从可信租户、任务、执行和动作生成固定长度键；它只承担重试幂等，不暴露内部自增主键或敏感元数据。
func auditDedupKey(tenantID uint64, taskPublicID, attemptPublicID, action string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%s", tenantID, taskPublicID, attemptPublicID, action)))
	return hex.EncodeToString(sum[:])
}

// frozenRecipientBindings 从服务端冻结快照构造“用户 + 公钥版本”集合；仅多接收者 RSA 快照需要完整集合校验。
// 返回空集合表示当前是兼容的单接收者协议，调用方继续交由算法适配器逐项校验。
func frozenRecipientBindings(snapshot map[string]any) map[string]struct{} {
	result := make(map[string]struct{})
	if stringValue(snapshot["type"]) != "RSA_RECIPIENTS" {
		return result
	}
	recipients, _ := snapshot["recipients"].([]any)
	for _, item := range recipients {
		recipient, ok := item.(map[string]any)
		if !ok {
			continue
		}
		identity := recipientBindingIdentity(mustUint64(recipient["recipient_user_id"]), stringValue(recipient["rsa_public_key_id"]))
		if identity != "" {
			result[identity] = struct{}{}
		}
	}
	return result
}

// submittedRecipientBinding 从客户端完成项读取接收者身份；它只读取标量标识，不信任客户端提供的公钥内容或指纹。
func submittedRecipientBinding(binding map[string]any) (string, bool) {
	if stringValue(binding["type"]) != "RSA_RECIPIENT" {
		return "", false
	}
	recipientID, ok := uint64Value(binding["recipient_user_id"])
	keyID := stringValue(binding["rsa_public_key_id"])
	identity := recipientBindingIdentity(recipientID, keyID)
	return identity, ok && identity != ""
}

// recipientBindingIdentity 生成仅用于一次完成请求内集合比较的稳定键，不写入数据库或审计日志。
func recipientBindingIdentity(recipientID uint64, keyID string) string {
	if recipientID == 0 || keyID == "" {
		return ""
	}
	return fmt.Sprintf("%d:%s", recipientID, keyID)
}

// firstPositive 兼容新旧 Benchmark 字段名；新字段为正数时优先，否则读取旧协议字段。
func firstPositive(preferred, legacy int64) int64 {
	if preferred > 0 {
		return preferred
	}
	return legacy
}

// firstPositiveOrZero 与 firstPositive 相同，但明确用于允许零毫秒的耗时指标。
func firstPositiveOrZero(preferred, legacy int64) int64 {
	if preferred != 0 {
		return preferred
	}
	return legacy
}

// Cancel 将可中断执行安全转换为取消终态并释放并发租约。
func (s *EncryptionService) Cancel(ctx context.Context, tenantID, actorUserID uint64, taskPublicID string) (repository.EncryptionTaskAggregate, error) {
	aggregate, err := s.repository.FindTask(ctx, tenantID, actorUserID, taskPublicID)
	if err != nil {
		return aggregate, mapEncryptionRepositoryError(err)
	}
	if aggregate.Task.Status == domain.EncryptionCancelled {
		return aggregate, nil
	}
	if err := s.repository.MarkTerminal(ctx, tenantID, actorUserID, taskPublicID, aggregate.Attempt.PublicID, domain.EncryptionCancelled, "USER_CANCELLED", false); err != nil {
		return aggregate, response.ErrEncryptionCancelRejected
	}
	s.admission.Release(ctx, tenantID, aggregate.Attempt.PublicID)
	_ = s.record(ctx, tenantID, actorUserID, "encryption.cancel", taskPublicID, "SUCCESS", "SERVER_OBSERVED", "", map[string]any{"task_id": taskPublicID, "attempt_id": aggregate.Attempt.PublicID})
	return s.repository.FindTask(ctx, tenantID, actorUserID, taskPublicID)
}

// Retry 为可恢复失败创建全新执行；授权快照和文件记录保持不可变。
func (s *EncryptionService) Retry(ctx context.Context, tenantID, actorUserID uint64, taskPublicID string) (repository.EncryptionTaskAggregate, error) {
	attemptID, err := identifier.NewUUID()
	if err != nil {
		return repository.EncryptionTaskAggregate{}, response.ErrInternal
	}
	if err := s.admission.Acquire(ctx, tenantID, actorUserID, attemptID); err != nil {
		return repository.EncryptionTaskAggregate{}, err
	}
	// 重试会新建执行记录，同样需要服务端时间戳保证持久化语义一致。
	now := time.Now()
	aggregate, err := s.repository.CreateRetry(ctx, tenantID, actorUserID, taskPublicID, domain.EncryptionTaskAttempt{PublicID: attemptID, Status: domain.EncryptionPending, StartedAt: now, UpdatedAt: now})
	if err != nil {
		s.admission.Release(ctx, tenantID, attemptID)
		return aggregate, response.ErrEncryptionRetryRejected
	}
	_ = s.record(ctx, tenantID, actorUserID, "encryption.retry", taskPublicID, "SUCCESS", "SERVER_OBSERVED", "", map[string]any{"task_id": taskPublicID, "attempt_id": attemptID, "attempt_no": aggregate.Attempt.AttemptNo})
	return aggregate, nil
}

// Fail 将执行转换为带稳定错误码的失败终态，供主进程异常上报复用。
func (s *EncryptionService) Fail(ctx context.Context, tenantID, actorUserID uint64, taskPublicID, attemptPublicID, failureCode string, retryable bool) error {
	if failureCode == "" {
		failureCode = "ENCRYPTION_EXECUTION_FAILED"
	}
	if err := s.repository.MarkTerminal(ctx, tenantID, actorUserID, taskPublicID, attemptPublicID, domain.EncryptionFailed, failureCode, retryable); err != nil {
		return mapEncryptionRepositoryError(err)
	}
	s.admission.Release(ctx, tenantID, attemptPublicID)
	recordAuditBestEffort(ctx, s.audit, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encryption.fail", TargetType: "encryption", TargetPublicID: taskPublicID, Result: "FAILURE", SourceTrust: "SERVER_OBSERVED", ErrorCode: failureCode, Metadata: map[string]any{"task_id": taskPublicID, "attempt_id": attemptPublicID, "retryable": retryable}})
	return nil
}

// algorithmEnabled 判断数据库租户目录是否精确启用请求算法版本。
func algorithmEnabled(algorithms []domain.EncryptionAlgorithm, code, version string) bool {
	for _, algorithm := range algorithms {
		if algorithm.Code == code && algorithm.Version == version {
			return true
		}
	}
	return false
}

// authorizationIncludesOwner 检查授权快照是否包含文件拥有者自己的受保护 DEK 目标。
//
// owner 身份只说明谁创建了文件，不等于拥有解密材料。若这里不强制包含 owner，
// 后续列表只能显示“不可解密”，且服务端无法在不知道明文 DEK 的情况下补救旧密文。
func authorizationIncludesOwner(snapshot map[string]any, ownerUserID uint64) bool {
	if ownerUserID == 0 {
		return false
	}
	if stringValue(snapshot["type"]) == "RSA_RECIPIENTS" {
		switch recipients := snapshot["recipients"].(type) {
		case []map[string]any:
			for _, recipient := range recipients {
				if mustUint64(recipient["recipient_user_id"]) == ownerUserID {
					return true
				}
			}
		case []any:
			for _, item := range recipients {
				recipient, ok := item.(map[string]any)
				if ok && mustUint64(recipient["recipient_user_id"]) == ownerUserID {
					return true
				}
			}
		}
		return false
	}
	return mustUint64(snapshot["recipient_user_id"]) == ownerUserID
}

// adapterKey 生成算法适配器精确版本键，未知版本不能回退到其他实现。
func adapterKey(code, version string) string { return code + "@" + version }

// copyAllowedAuthorizationAudit 从适配器快照复制审计白名单字段，不复制公钥材料或任意扩展 JSON。
func copyAllowedAuthorizationAudit(target, snapshot map[string]any) {
	for _, key := range []string{"recipient_user_id", "rsa_public_key_id", "public_key_version", "public_key_fingerprint_sha256"} {
		if value, ok := snapshot[key]; ok {
			auditKey := key
			if key == "rsa_public_key_id" {
				auditKey = "public_key_id"
			}
			if key == "public_key_fingerprint_sha256" {
				auditKey = "fingerprint_sha256"
			}
			target[auditKey] = value
		}
	}
}

// mapEncryptionRepositoryError 将内部仓储错误映射为稳定且不泄漏租户存在性的业务错误。
func mapEncryptionRepositoryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, repository.ErrEncryptionTaskNotFound):
		return response.ErrEncryptionTaskNotFound
	case errors.Is(err, repository.ErrEncryptionAttemptNotFound):
		return response.ErrEncryptionAttemptNotFound
	case errors.Is(err, repository.ErrEncryptedFileNotFound):
		return response.ErrEncryptedFileNotFound
	case errors.Is(err, repository.ErrEncryptionStateConflict):
		return response.ErrEncryptionStateConflict
	default:
		return response.ErrInternal
	}
}

// record 写入加密链路审计；调用方决定安全关键事件失败是否阻断响应。
func (s *EncryptionService) record(ctx context.Context, tenantID, actorUserID uint64, action, targetPublicID, result, sourceTrust, errorCode string, metadata map[string]any) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: action, TargetType: "encryption", TargetPublicID: targetPublicID, Result: result, SourceTrust: sourceTrust, ErrorCode: errorCode, Metadata: metadata})
}

// sameStagingUpload 比较服务端已复核的暂存事实与重试声明；仅格式、大小和摘要全部一致时才允许复用。
func sameStagingUpload(existing domain.CiphertextObject, format, expectedSHA256 string, contentLength int64) bool {
	return existing.ContainerFormat == format && existing.CiphertextSize == contentLength && strings.EqualFold(existing.CiphertextSHA256, expectedSHA256)
}

// zeroSensitiveBytes 尽力覆盖服务端短暂解码的受保护密钥副本；该数据不是明文 DEK，但仍按敏感材料处理。
func zeroSensitiveBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
