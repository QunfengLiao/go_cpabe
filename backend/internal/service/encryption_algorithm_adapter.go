package service

import (
	"context"
	"strings"

	cryptomodule "go-cpabe/backend/internal/crypto"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
)

// EncryptionAlgorithmAdapter 隔离算法授权校验和专属完成绑定，使任务、AES、上传与审计保持通用。
type EncryptionAlgorithmAdapter interface {
	// Code 返回稳定算法编码。
	Code() string
	// Version 返回适配器协议版本。
	Version() string
	// AuthorizationType 返回前端动态表单类型。
	AuthorizationType() string
	// ValidateAuthorization 校验租户内授权并返回不含秘密材料的不可变快照。
	ValidateAuthorization(ctx context.Context, tenantID uint64, raw map[string]any) (map[string]any, error)
	// ValidateCompletion 校验专属绑定并返回通用元数据校验输入与持久化计划。
	ValidateCompletion(ctx context.Context, tenantID uint64, authorizationSnapshot map[string]any, protected cryptomodule.ProtectedKeyResult, raw map[string]any) (cryptomodule.ProtectedKeyResult, repository.EncryptionAdapterBinding, error)
}

// RSAEncryptionAlgorithmAdapter 是首个真实算法适配器，RSA 专属字段不进入通用 EncryptionService。
type RSAEncryptionAlgorithmAdapter struct{ keys *RSAKeyService }

// NewRSAEncryptionAlgorithmAdapter 创建 RSA 授权和完成绑定适配器。
func NewRSAEncryptionAlgorithmAdapter(keys *RSAKeyService) *RSAEncryptionAlgorithmAdapter {
	return &RSAEncryptionAlgorithmAdapter{keys: keys}
}

// Code 返回 RSA-OAEP-SHA256 编码。
func (RSAEncryptionAlgorithmAdapter) Code() string { return cryptomodule.AlgorithmRSAOAEP256 }

// Version 返回首期 RSA 协议版本。
func (RSAEncryptionAlgorithmAdapter) Version() string { return cryptomodule.AlgorithmVersion1 }

// AuthorizationType 返回 RSA 接收者表单类型。
func (RSAEncryptionAlgorithmAdapter) AuthorizationType() string { return "RSA_RECIPIENT" }

// ValidateAuthorization 校验接收者和同租户 ACTIVE 公钥后冻结版本与指纹。
func (a *RSAEncryptionAlgorithmAdapter) ValidateAuthorization(ctx context.Context, tenantID uint64, raw map[string]any) (map[string]any, error) {
	switch stringValue(raw["type"]) {
	case "RSA_RECIPIENTS":
		return a.validateRecipientSet(ctx, tenantID, raw)
	case a.AuthorizationType():
	default:
		return nil, response.ErrEncryptionAlgorithmUnavailable
	}
	recipientID, ok := uint64Value(raw["recipient_user_id"])
	if !ok {
		return nil, response.ErrRSAKeyNotFound
	}
	key, err := a.keys.ActiveKey(ctx, tenantID, stringValue(raw["rsa_public_key_id"]))
	if err != nil || key.UserID != recipientID {
		return nil, response.ErrRSAKeyNotFound
	}
	return map[string]any{"type": a.AuthorizationType(), "recipient_user_id": key.UserID, "rsa_public_key_id": key.PublicID, "public_key_version": key.Version, "public_key_fingerprint_sha256": key.FingerprintSHA256}, nil
}

// ValidateCompletion 重查公钥版本并构造 RSA 专属持久化计划，历史绑定不会静默切换版本。
func (a *RSAEncryptionAlgorithmAdapter) ValidateCompletion(ctx context.Context, tenantID uint64, authorizationSnapshot map[string]any, protected cryptomodule.ProtectedKeyResult, raw map[string]any) (cryptomodule.ProtectedKeyResult, repository.EncryptionAdapterBinding, error) {
	if stringValue(raw["type"]) != a.AuthorizationType() {
		return protected, nil, response.ErrProtectedKeyInvalid
	}
	recipientID, ok := uint64Value(raw["recipient_user_id"])
	if !ok {
		return protected, nil, response.ErrProtectedKeyInvalid
	}
	keyPublicID := stringValue(raw["rsa_public_key_id"])
	fingerprint := stringValue(raw["public_key_fingerprint_sha256"])
	frozen, ok := findFrozenRSARecipient(authorizationSnapshot, recipientID, keyPublicID)
	if !ok || !strings.EqualFold(fingerprint, stringValue(frozen["public_key_fingerprint_sha256"])) {
		return protected, nil, response.ErrProtectedKeyInvalid
	}
	key, err := a.keys.KeyVersion(ctx, tenantID, keyPublicID)
	if err != nil || key.UserID != recipientID || key.Version != uint32(mustUint64(frozen["public_key_version"])) || !strings.EqualFold(key.FingerprintSHA256, fingerprint) {
		return protected, nil, response.ErrRSAKeyNotFound
	}
	if stringValue(raw["oaep_hash"]) != "SHA-256" || stringValue(raw["oaep_label_sha256"]) != protected.ContextSHA256 {
		return protected, nil, response.ErrProtectedKeyInvalid
	}
	protected.Binding = map[string]any{"type": a.AuthorizationType(), "rsa_public_key_id": key.ID}
	plan := repository.RSAEncryptionAdapterBinding{Binding: domain.RSAProtectedKeyBinding{RecipientUserID: key.UserID, RSAPublicKeyID: key.ID, PublicKeyFingerprintSHA256: key.FingerprintSHA256, OAEPHash: "SHA-256", OAEPLabelSHA256: protected.ContextSHA256, ProtectDurationMS: int64Value(raw["protect_duration_ms"])}}
	return protected, plan, nil
}

// validateRecipientSet 校验 RSA_RECIPIENTS 数组并冻结每个接收者的公钥版本与指纹。
func (a *RSAEncryptionAlgorithmAdapter) validateRecipientSet(ctx context.Context, tenantID uint64, raw map[string]any) (map[string]any, error) {
	rawRecipients, ok := raw["recipients"].([]any)
	if !ok || len(rawRecipients) == 0 {
		return nil, response.ErrRSAKeyNotFound
	}
	seen := make(map[uint64]struct{}, len(rawRecipients))
	recipients := make([]map[string]any, 0, len(rawRecipients))
	for _, item := range rawRecipients {
		recipient, ok := item.(map[string]any)
		if !ok {
			return nil, response.ErrRSAKeyNotFound
		}
		recipientID, ok := uint64Value(recipient["user_id"])
		if !ok {
			recipientID, ok = uint64Value(recipient["recipient_user_id"])
		}
		if !ok {
			return nil, response.ErrRSAKeyNotFound
		}
		if _, exists := seen[recipientID]; exists {
			return nil, response.ErrProtectedKeyInvalid
		}
		seen[recipientID] = struct{}{}
		keyID := stringValue(recipient["public_key_id"])
		if keyID == "" {
			keyID = stringValue(recipient["rsa_public_key_id"])
		}
		key, err := a.keys.ActiveKey(ctx, tenantID, keyID)
		if err != nil || key.UserID != recipientID {
			return nil, response.ErrRSAKeyNotFound
		}
		recipients = append(recipients, map[string]any{"type": a.AuthorizationType(), "recipient_user_id": key.UserID, "rsa_public_key_id": key.PublicID, "public_key_version": key.Version, "public_key_fingerprint_sha256": key.FingerprintSHA256})
	}
	return map[string]any{"type": "RSA_RECIPIENTS", "recipients": recipients, "recipient_count": len(recipients)}, nil
}

// findFrozenRSARecipient 在旧单接收者和新多接收者快照中查找目标绑定。
func findFrozenRSARecipient(snapshot map[string]any, recipientID uint64, keyPublicID string) (map[string]any, bool) {
	if stringValue(snapshot["type"]) == "RSA_RECIPIENTS" {
		recipients, _ := snapshot["recipients"].([]any)
		for _, item := range recipients {
			recipient, ok := item.(map[string]any)
			if ok && mustUint64(recipient["recipient_user_id"]) == recipientID && stringValue(recipient["rsa_public_key_id"]) == keyPublicID {
				return recipient, true
			}
		}
		return nil, false
	}
	if recipientID == mustUint64(snapshot["recipient_user_id"]) && keyPublicID == stringValue(snapshot["rsa_public_key_id"]) {
		return snapshot, true
	}
	return nil, false
}

// mustUint64 读取已由服务端生成的不可变快照；缺失或类型异常返回零并使后续绑定比较失败。
func mustUint64(value any) uint64 {
	number, _ := uint64Value(value)
	return number
}

// stringValue 安全读取 JSON 标量字符串，未知类型返回空值。
func stringValue(value any) string { text, _ := value.(string); return strings.TrimSpace(text) }

// uint64Value 兼容 JSON 解码的 float64 与服务内部测试的整数类型，拒绝负数和小数。
func uint64Value(value any) (uint64, bool) {
	switch number := value.(type) {
	case uint64:
		return number, number > 0
	case uint32:
		return uint64(number), number > 0
	case int:
		return uint64(number), number > 0
	case int64:
		return uint64(number), number > 0
	case float64:
		converted := uint64(number)
		return converted, number > 0 && float64(converted) == number
	default:
		return 0, false
	}
}

// int64Value 读取客户端报告的非敏感耗时标量，异常值按零处理避免影响安全决策。
func int64Value(value any) int64 {
	switch number := value.(type) {
	case int64:
		if number > 0 {
			return number
		}
	case int:
		if number > 0 {
			return int64(number)
		}
	case uint64:
		return int64(number)
	case float64:
		if number > 0 {
			return int64(number)
		}
	}
	return 0
}
