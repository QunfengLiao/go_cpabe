package crypto

import (
	"encoding/hex"
	"errors"
)

// ProtectedKeyMetadataValidator 定义单一算法版本的受保护密钥元数据校验器。
type ProtectedKeyMetadataValidator interface {
	// Validate 校验格式、上下文和专属绑定，不执行解密。
	Validate(result ProtectedKeyResult) error
}

// MetadataValidator 按算法 code/version 分派专属校验器，通用完成事务无需 RSA 分支。
type MetadataValidator struct {
	validators map[string]ProtectedKeyMetadataValidator
}

// NewMetadataValidator 创建产品校验器目录，首期仅注册真实 RSA 适配器。
func NewMetadataValidator() *MetadataValidator {
	return &MetadataValidator{validators: map[string]ProtectedKeyMetadataValidator{AlgorithmRSAOAEP256 + "@" + AlgorithmVersion1: rsaMetadataValidator{}}}
}

// Register 仅供测试或未来真实算法接入时显式注册，禁止覆盖既有版本。
func (v *MetadataValidator) Register(code, version string, validator ProtectedKeyMetadataValidator) error {
	if v == nil || validator == nil || code == "" || version == "" {
		return errors.New("invalid metadata validator")
	}
	if v.validators == nil {
		v.validators = make(map[string]ProtectedKeyMetadataValidator)
	}
	key := code + "@" + version
	if _, exists := v.validators[key]; exists {
		return errors.New("duplicate metadata validator")
	}
	v.validators[key] = validator
	return nil
}

// ValidateProtectedKey 精确分派算法版本，未知算法不得回退到 RSA 校验。
func (v *MetadataValidator) ValidateProtectedKey(result ProtectedKeyResult) error {
	if v == nil {
		return errors.New("metadata validator is required")
	}
	validator, ok := v.validators[result.AlgorithmCode+"@"+result.AlgorithmVersion]
	if !ok {
		return errors.New("unsupported protected key algorithm")
	}
	return validator.Validate(result)
}

// rsaMetadataValidator 隔离首期 RSA 格式和绑定规则。
type rsaMetadataValidator struct{}

// Validate 校验 RSA-OAEP-SHA256 受保护 DEK 的固定长度和专属绑定存在性。
func (rsaMetadataValidator) Validate(result ProtectedKeyResult) error {
	if result.Format != "RSA-OAEP-SHA256-RAW" || len(result.Value) != 384 {
		return errors.New("invalid protected key format")
	}
	contextHash, err := hex.DecodeString(result.ContextSHA256)
	if err != nil || len(contextHash) != 32 {
		return errors.New("invalid protected key context")
	}
	if result.Binding["type"] != "RSA_RECIPIENT" || result.Binding["rsa_public_key_id"] == nil {
		return errors.New("invalid RSA binding")
	}
	return nil
}
