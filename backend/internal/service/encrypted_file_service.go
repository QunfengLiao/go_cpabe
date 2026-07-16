package service

import (
	"context"
	"encoding/base64"
	"io"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/pkg/storage"
	"go-cpabe/backend/internal/repository"
)

// EncryptedFileDownload 是完成密文的鉴权读取结果，对象键不向 Handler 暴露。
type EncryptedFileDownload struct {
	Reader   io.ReadCloser
	Filename string
	Size     int64
	SHA256   string
}

// ReceivedDecryptionMaterial 是随可见密文返回的本地解密材料，不包含私钥或明文 DEK。
type ReceivedDecryptionMaterial struct {
	FileID                     string `json:"file_id"`
	OriginalFilename           string `json:"original_filename"`
	PlaintextSize              int64  `json:"plaintext_size"`
	AlgorithmCode              string `json:"algorithm_code"`
	AlgorithmVersion           string `json:"algorithm_version"`
	ProtectedKeyFormat         string `json:"protected_key_format"`
	ProtectedKeyBase64         string `json:"protected_key_base64"`
	ContextSHA256              string `json:"context_sha256"`
	RSAPublicKeyID             string `json:"rsa_public_key_id"`
	PublicKeyFingerprintSHA256 string `json:"public_key_fingerprint_sha256"`
	// KeyEnvelopes 是面向不同 RSA 公钥的受保护 DEK 集合；客户端必须在本地匹配私钥。
	KeyEnvelopes []KeyEnvelope `json:"key_envelopes"`
}

// KeyEnvelope 表示一个可公开传输但无法单独解密的 RSA 密钥信封。
type KeyEnvelope struct {
	KeyID                      string `json:"key_id"`
	ProtectedKeyBase64         string `json:"protected_key_base64"`
	ContextSHA256              string `json:"context_sha256"`
	AlgorithmCode              string `json:"algorithm_code"`
	AlgorithmVersion           string `json:"algorithm_version"`
	ProtectedKeyFormat         string `json:"protected_key_format"`
	RSAPublicKeyID             string `json:"rsa_public_key_id"`
	PublicKeyFingerprintSHA256 string `json:"public_key_fingerprint_sha256"`
	OAEPHash                   string `json:"oaep_hash"`
}

// FileCenterScope 表示文件中心的密文列表范围；列表可见性不表达本地解密能力。
type FileCenterScope string

const (
	// FileCenterTenantCloud 表示当前租户企业云盘，可见可用文件元数据和密文下载入口。
	FileCenterTenantCloud FileCenterScope = "tenant_cloud"
	// FileCenterOwnedByMe 表示当前用户作为 DO 创建的文件。
	FileCenterOwnedByMe FileCenterScope = "owned_by_me"
)

// EncryptedFileService 提供 DO 自有文件查询和已完成密文下载授权。
type EncryptedFileService struct {
	repository repository.EncryptionRepository
	storage    storage.EncryptedFileStorage
	audit      AuditRecorder
}

// NewEncryptedFileService 创建自有加密文件服务。
func NewEncryptedFileService(repository repository.EncryptionRepository, encryptedStorage storage.EncryptedFileStorage, audit AuditRecorder) *EncryptedFileService {
	return &EncryptedFileService{repository: repository, storage: encryptedStorage, audit: audit}
}

// ListOwned 返回可信租户和当前 DO 双重约束的分页文件。
func (s *EncryptedFileService) ListOwned(ctx context.Context, tenantID, actorUserID uint64, status domain.EncryptedFileStatus, page, pageSize int) (repository.EncryptedFilePage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	result, err := s.repository.ListOwnedFiles(ctx, tenantID, actorUserID, status, (page-1)*pageSize, pageSize)
	if err != nil {
		return result, response.ErrInternal
	}
	return result, nil
}

// ListReceived 返回当前租户可见的可用文件兼容列表；是否拥有匹配信封由客户端本地判断。
func (s *EncryptedFileService) ListReceived(ctx context.Context, tenantID, actorUserID uint64, page, pageSize int) (repository.EncryptedFilePage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	result, err := s.repository.ListReceivedFiles(ctx, tenantID, actorUserID, (page-1)*pageSize, pageSize)
	if err != nil {
		return result, response.ErrInternal
	}
	return result, nil
}

// ListFileCenter 按文件中心 scope 聚合查询；企业云盘只返回元数据，不包含解密材料。
func (s *EncryptedFileService) ListFileCenter(ctx context.Context, tenantID, actorUserID uint64, scope FileCenterScope, page, pageSize int) (repository.EncryptedFilePage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	switch scope {
	case FileCenterTenantCloud:
		result, err := s.repository.ListTenantFiles(ctx, tenantID, actorUserID, domain.EncryptedFileAvailable, (page-1)*pageSize, pageSize)
		if err != nil {
			return result, response.ErrInternal
		}
		return result, nil
	case FileCenterOwnedByMe:
		return s.ListOwned(ctx, tenantID, actorUserID, "", page, pageSize)
	default:
		return repository.EncryptedFilePage{}, response.ErrBadRequest
	}
}

// FileCenterDetail 返回当前租户可见文件的统一详情；仓储只装配展示摘要，不在此路径返回任何 protected DEK。
func (s *EncryptedFileService) FileCenterDetail(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (repository.EncryptedFileDetail, error) {
	detail, err := s.repository.FindTenantFile(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return detail, mapEncryptionRepositoryError(err)
	}
	return detail, nil
}

// DownloadFileCenter 允许同租户有效成员下载 AVAILABLE 原始密文；服务端不判断本地私钥或解密能力。
func (s *EncryptedFileService) DownloadFileCenter(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (EncryptedFileDownload, error) {
	detail, err := s.repository.FindTenantFile(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return EncryptedFileDownload{}, mapEncryptionRepositoryError(err)
	}
	if detail.File.Status != domain.EncryptedFileAvailable || detail.Object == nil || detail.Object.Status != domain.CiphertextAvailable {
		return EncryptedFileDownload{}, response.ErrEncryptedFileUnavailable
	}
	reader, err := s.storage.OpenCiphertext(ctx, detail.Object.ObjectKey)
	if err != nil {
		return EncryptedFileDownload{}, response.ErrCiphertextStorageFailed
	}
	if err := s.record(ctx, tenantID, actorUserID, filePublicID, "SUCCESS", ""); err != nil {
		_ = reader.Close()
		return EncryptedFileDownload{}, response.ErrInternal
	}
	return EncryptedFileDownload{Reader: reader, Filename: detail.File.OriginalFilename + ".enc", Size: detail.Object.CiphertextSize, SHA256: detail.Object.CiphertextSHA256}, nil
}

// OwnDecryptionMaterial 返回可见文件的全部密钥信封，不能把 RBAC 角色当作解密授权。
func (s *EncryptedFileService) OwnDecryptionMaterial(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (ReceivedDecryptionMaterial, error) {
	return s.ReceivedMaterial(ctx, tenantID, actorUserID, filePublicID)
}

// ReceivedMaterial 返回可见文件的全部受保护 DEK 与公钥外部标识，客户端负责匹配本地私钥。
func (s *EncryptedFileService) ReceivedMaterial(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (ReceivedDecryptionMaterial, error) {
	detail, err := s.repository.FindReceivedFile(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		return ReceivedDecryptionMaterial{}, mapEncryptionRepositoryError(err)
	}
	envelopes := make([]KeyEnvelope, 0, len(detail.KeyEnvelopes))
	for _, envelope := range detail.KeyEnvelopes {
		envelopes = append(envelopes, KeyEnvelope{
			KeyID:                      envelope.ProtectedKey.PublicID,
			ProtectedKeyBase64:         base64.StdEncoding.EncodeToString(envelope.ProtectedKey.ProtectedKeyBytes),
			ContextSHA256:              envelope.ProtectedKey.ContextSHA256,
			AlgorithmCode:              envelope.ProtectedKey.AlgorithmCode,
			AlgorithmVersion:           envelope.ProtectedKey.AlgorithmVersion,
			ProtectedKeyFormat:         envelope.ProtectedKey.ProtectedKeyFormat,
			RSAPublicKeyID:             envelope.PublicKey.PublicID,
			PublicKeyFingerprintSHA256: envelope.Binding.PublicKeyFingerprintSHA256,
			OAEPHash:                   envelope.Binding.OAEPHash,
		})
	}
	// 测试替身和旧数据可能只有单条聚合字段，保留兼容回退但不按当前用户筛选。
	if len(envelopes) == 0 && detail.ProtectedKey != nil && detail.RSABinding != nil && detail.RSAPublicKey != nil {
		envelopes = append(envelopes, KeyEnvelope{KeyID: detail.ProtectedKey.PublicID, ProtectedKeyBase64: base64.StdEncoding.EncodeToString(detail.ProtectedKey.ProtectedKeyBytes), ContextSHA256: detail.ProtectedKey.ContextSHA256, AlgorithmCode: detail.ProtectedKey.AlgorithmCode, AlgorithmVersion: detail.ProtectedKey.AlgorithmVersion, ProtectedKeyFormat: detail.ProtectedKey.ProtectedKeyFormat, RSAPublicKeyID: detail.RSAPublicKey.PublicID, PublicKeyFingerprintSHA256: detail.RSABinding.PublicKeyFingerprintSHA256, OAEPHash: detail.RSABinding.OAEPHash})
	}
	first := KeyEnvelope{AlgorithmCode: detail.Task.AlgorithmCode, AlgorithmVersion: detail.Task.AlgorithmVersion}
	if len(envelopes) > 0 {
		first = envelopes[0]
	}
	return ReceivedDecryptionMaterial{FileID: detail.File.PublicID, OriginalFilename: detail.File.OriginalFilename, PlaintextSize: detail.File.PlaintextSize, AlgorithmCode: first.AlgorithmCode, AlgorithmVersion: first.AlgorithmVersion, ProtectedKeyFormat: first.ProtectedKeyFormat, ProtectedKeyBase64: first.ProtectedKeyBase64, ContextSHA256: first.ContextSHA256, RSAPublicKeyID: first.RSAPublicKeyID, PublicKeyFingerprintSHA256: first.PublicKeyFingerprintSHA256, KeyEnvelopes: envelopes}, nil
}

// DownloadReceived 向当前租户可见用户流式返回可用密文，下载不等于解密成功。
func (s *EncryptedFileService) DownloadReceived(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (EncryptedFileDownload, error) {
	detail, err := s.repository.FindReceivedFile(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		s.recordReceived(ctx, tenantID, actorUserID, filePublicID, "DENIED", response.ErrEncryptedFileNotFound.Code)
		return EncryptedFileDownload{}, mapEncryptionRepositoryError(err)
	}
	if detail.Object == nil || detail.Object.Status != domain.CiphertextAvailable {
		return EncryptedFileDownload{}, response.ErrEncryptedFileUnavailable
	}
	reader, err := s.storage.OpenCiphertext(ctx, detail.Object.ObjectKey)
	if err != nil {
		return EncryptedFileDownload{}, response.ErrCiphertextStorageFailed
	}
	if err := s.recordReceived(ctx, tenantID, actorUserID, filePublicID, "SUCCESS", ""); err != nil {
		_ = reader.Close()
		return EncryptedFileDownload{}, response.ErrInternal
	}
	return EncryptedFileDownload{Reader: reader, Filename: detail.File.OriginalFilename + ".enc", Size: detail.Object.CiphertextSize, SHA256: detail.Object.CiphertextSHA256}, nil
}

// recordReceived 写入接收者密文读取事件，不记录受保护 DEK、私钥或本地保存路径。
func (s *EncryptedFileService) recordReceived(ctx context.Context, tenantID, actorUserID uint64, filePublicID, result, errorCode string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: "received_file.download", TargetType: "encrypted_file", TargetPublicID: filePublicID, Result: result, SourceTrust: "SERVER_OBSERVED", ErrorCode: errorCode, Metadata: map[string]any{"file_id": filePublicID}})
}

// Detail 返回脱敏文件详情；ProtectedKeyBytes 和 ObjectKey 通过 JSON 标签和 DTO 边界隐藏。
func (s *EncryptedFileService) Detail(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (repository.EncryptedFileDetail, error) {
	detail, err := s.repository.FindOwnedFile(ctx, tenantID, actorUserID, filePublicID)
	return detail, mapEncryptionRepositoryError(err)
}

// Download 仅允许所有者读取 AVAILABLE 文件的 AVAILABLE 密文对象。
func (s *EncryptedFileService) Download(ctx context.Context, tenantID, actorUserID uint64, filePublicID string) (EncryptedFileDownload, error) {
	detail, err := s.repository.FindOwnedFile(ctx, tenantID, actorUserID, filePublicID)
	if err != nil {
		s.record(ctx, tenantID, actorUserID, filePublicID, "DENIED", response.ErrEncryptedFileNotFound.Code)
		return EncryptedFileDownload{}, mapEncryptionRepositoryError(err)
	}
	if detail.File.Status != domain.EncryptedFileAvailable || detail.Object == nil || detail.Object.Status != domain.CiphertextAvailable {
		s.record(ctx, tenantID, actorUserID, filePublicID, "DENIED", response.ErrEncryptedFileUnavailable.Code)
		return EncryptedFileDownload{}, response.ErrEncryptedFileUnavailable
	}
	reader, err := s.storage.OpenCiphertext(ctx, detail.Object.ObjectKey)
	if err != nil {
		s.record(ctx, tenantID, actorUserID, filePublicID, "FAILURE", response.ErrCiphertextStorageFailed.Code)
		return EncryptedFileDownload{}, response.ErrCiphertextStorageFailed
	}
	if err := s.record(ctx, tenantID, actorUserID, filePublicID, "SUCCESS", ""); err != nil {
		_ = reader.Close()
		return EncryptedFileDownload{}, response.ErrInternal
	}
	return EncryptedFileDownload{Reader: reader, Filename: detail.File.OriginalFilename + ".enc", Size: detail.Object.CiphertextSize, SHA256: detail.Object.CiphertextSHA256}, nil
}

// record 写入下载成功、拒绝或存储异常事件。
func (s *EncryptedFileService) record(ctx context.Context, tenantID, actorUserID uint64, filePublicID, result, errorCode string) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: "encrypted_file.download", TargetType: "encrypted_file", TargetPublicID: filePublicID, Result: result, SourceTrust: "SERVER_OBSERVED", ErrorCode: errorCode, Metadata: map[string]any{"file_id": filePublicID}})
}
