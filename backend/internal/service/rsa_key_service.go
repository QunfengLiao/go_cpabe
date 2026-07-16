package service

import (
	"context"
	"errors"
	"strings"

	cryptomodule "go-cpabe/backend/internal/crypto"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/identifier"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// RegisterRSAKeyInput 是成员登记本人 SPKI 公钥的请求。
type RegisterRSAKeyInput struct {
	PublicKeyPEM string `json:"public_key_pem" binding:"required"`
}

// UpdateRSAKeyStatusInput 是管理员变更公钥状态的请求。
type UpdateRSAKeyStatusInput struct {
	Status string `json:"status" binding:"required"`
}

// RSAKeyService 管理租户成员公钥历史；该服务永不接收或保存 RSA 私钥。
type RSAKeyService struct {
	keys  repository.RSAKeyRepository
	audit AuditRecorder
}

// NewRSAKeyService 创建 RSA 公钥服务。
func NewRSAKeyService(keys repository.RSAKeyRepository, audit AuditRecorder) *RSAKeyService {
	return &RSAKeyService{keys: keys, audit: audit}
}

// RegisterMyKey 解析 SPKI、重算指纹并为当前成员登记 3072 位 RSA 公钥。
func (s *RSAKeyService) RegisterMyKey(ctx context.Context, tenantID, actorUserID uint64, input RegisterRSAKeyInput) (domain.RSAPublicKey, bool, error) {
	publicKey, fingerprint, err := cryptomodule.ParseRSAPublicKey(strings.TrimSpace(input.PublicKeyPEM))
	if err != nil || publicKey.N.BitLen() != 3072 {
		s.record(ctx, tenantID, actorUserID, "rsa_key.register", "", "FAILURE", response.ErrRSAKeyInvalid.Code, nil)
		return domain.RSAPublicKey{}, false, response.ErrRSAKeyInvalid
	}
	publicID, err := identifier.NewUUID()
	if err != nil {
		return domain.RSAPublicKey{}, false, response.ErrInternal
	}
	key := domain.RSAPublicKey{PublicID: publicID, TenantID: tenantID, UserID: actorUserID, FingerprintSHA256: fingerprint, PublicKeyPEM: strings.TrimSpace(input.PublicKeyPEM), KeyBits: uint16(publicKey.N.BitLen()), Algorithm: cryptomodule.AlgorithmRSAOAEP256, Status: "ACTIVE", CreatedBy: actorUserID}
	created, idempotent, err := s.keys.CreateVersion(ctx, key)
	if errors.Is(err, repository.ErrRSAKeyFingerprintExists) {
		return domain.RSAPublicKey{}, false, response.ErrRSAKeyFingerprintExists
	}
	if err != nil {
		return domain.RSAPublicKey{}, false, response.ErrInternal
	}
	if err := s.record(ctx, tenantID, actorUserID, "rsa_key.register", created.PublicID, "SUCCESS", "", map[string]any{"public_key_id": created.PublicID, "public_key_version": created.Version, "fingerprint_sha256": created.FingerprintSHA256}); err != nil {
		return domain.RSAPublicKey{}, false, response.ErrInternal
	}
	return created, idempotent, nil
}

// MyKeys 返回当前成员在当前租户的公钥版本历史。
func (s *RSAKeyService) MyKeys(ctx context.Context, tenantID, actorUserID uint64) ([]domain.RSAPublicKey, error) {
	keys, err := s.keys.ListUserKeys(ctx, tenantID, actorUserID)
	if err != nil {
		return nil, response.ErrInternal
	}
	return keys, nil
}

// Recipients 返回当前租户有效接收者及其有效公钥，并审计选择目录读取。
func (s *RSAKeyService) Recipients(ctx context.Context, tenantID, actorUserID uint64) ([]repository.RSARecipient, error) {
	recipients, err := s.keys.ListRecipients(ctx, tenantID)
	if err != nil {
		return nil, response.ErrInternal
	}
	if err := s.record(ctx, tenantID, actorUserID, "rsa_key.recipients.list", "", "SUCCESS", "", nil); err != nil {
		return nil, response.ErrInternal
	}
	return recipients, nil
}

// UpdateStatus 允许租户管理员禁用、撤销或重新启用公钥版本，历史绑定保持可解释。
func (s *RSAKeyService) UpdateStatus(ctx context.Context, tenantID, actorUserID uint64, keyPublicID string, input UpdateRSAKeyStatusInput) (domain.RSAPublicKey, error) {
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status != "ACTIVE" && status != "DISABLED" && status != "REVOKED" {
		return domain.RSAPublicKey{}, response.ErrBadRequest
	}
	key, err := s.keys.UpdateStatus(ctx, tenantID, keyPublicID, status, actorUserID)
	if errors.Is(err, repository.ErrRSAKeyNotFound) {
		return domain.RSAPublicKey{}, response.ErrRSAKeyNotFound
	}
	if err != nil {
		return domain.RSAPublicKey{}, response.ErrInternal
	}
	if err := s.record(ctx, tenantID, actorUserID, "rsa_key.status.update", key.PublicID, "SUCCESS", "", map[string]any{"public_key_id": key.PublicID, "status": status}); err != nil {
		return domain.RSAPublicKey{}, response.ErrInternal
	}
	return key, nil
}

// ActiveKey 返回任务创建时冻结的同租户可用公钥事实。
func (s *RSAKeyService) ActiveKey(ctx context.Context, tenantID uint64, keyPublicID string) (domain.RSAPublicKey, error) {
	key, err := s.keys.FindActiveKey(ctx, tenantID, keyPublicID)
	if errors.Is(err, repository.ErrRSAKeyNotFound) {
		return key, response.ErrRSAKeyNotFound
	}
	if err != nil {
		return key, response.ErrInternal
	}
	return key, nil
}

// KeyVersion 返回租户内已登记的历史公钥版本，供既有任务校验冻结绑定，不允许用于创建新任务。
func (s *RSAKeyService) KeyVersion(ctx context.Context, tenantID uint64, keyPublicID string) (domain.RSAPublicKey, error) {
	key, err := s.keys.FindKey(ctx, tenantID, keyPublicID)
	if errors.Is(err, repository.ErrRSAKeyNotFound) {
		return key, response.ErrRSAKeyNotFound
	}
	if err != nil {
		return key, response.ErrInternal
	}
	return key, nil
}

// record 写入 RSA 安全事件；登记和状态变更的审计失败会阻断对外成功响应。
func (s *RSAKeyService) record(ctx context.Context, tenantID, actorUserID uint64, action, targetPublicID, result, errorCode string, metadata map[string]any) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.Record(ctx, AuditEvent{TenantID: &tenantID, ActorUserID: actorUserID, Action: action, TargetType: "rsa_public_key", TargetPublicID: targetPublicID, Result: result, SourceTrust: "SERVER_OBSERVED", ErrorCode: errorCode, Metadata: metadata})
}
