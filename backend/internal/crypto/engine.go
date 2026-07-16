package crypto

import (
	"context"
	"errors"
)

const (
	// AlgorithmRSAOAEP256 是本期唯一启用的 DEK 保护算法编码。
	AlgorithmRSAOAEP256 = "RSA-OAEP-SHA256"
	// AlgorithmVersion1 是首期 RSA 和容器协议版本。
	AlgorithmVersion1 = "1"
)

// Authorization 描述算法已验证的授权输入；通用层不得读取 RSA 专属字段。
type Authorization struct {
	Type       string         `json:"type"`
	Parameters map[string]any `json:"parameters"`
}

// ProtectedKeyResult 是 DEK 保护引擎的算法无关输出，不包含明文 DEK。
type ProtectedKeyResult struct {
	AlgorithmCode    string         `json:"algorithm_code"`
	AlgorithmVersion string         `json:"algorithm_version"`
	Format           string         `json:"format"`
	Value            []byte         `json:"-"`
	ContextSHA256    string         `json:"context_sha256"`
	Binding          map[string]any `json:"binding"`
}

// DEKProtector 定义算法模块保护 DEK 所需的最小能力。
type DEKProtector interface {
	// Code 返回算法稳定编码。
	Code() string
	// Version 返回算法参数协议版本。
	Version() string
	// Protect 使用经验证授权保护 DEK，并返回算法无关结果。
	Protect(ctx context.Context, dek []byte, authorization Authorization, contextHash []byte) (ProtectedKeyResult, error)
}

// Registry 保存本地 Crypto Worker 可调用的真实 DEK 保护引擎。
type Registry struct {
	protectors map[string]DEKProtector
}

// NewRegistry 创建空引擎注册表；产品启动必须显式注册允许的真实实现。
func NewRegistry() *Registry {
	return &Registry{protectors: make(map[string]DEKProtector)}
}

// Register 注册一个算法版本，重复编码会返回错误以避免静默覆盖安全参数。
func (r *Registry) Register(protector DEKProtector) error {
	if protector == nil || protector.Code() == "" || protector.Version() == "" {
		return errors.New("invalid DEK protector")
	}
	key := protector.Code() + "@" + protector.Version()
	if _, exists := r.protectors[key]; exists {
		return errors.New("duplicate DEK protector")
	}
	r.protectors[key] = protector
	return nil
}

// Resolve 返回精确算法版本，不允许未知算法回退到 RSA。
func (r *Registry) Resolve(code, version string) (DEKProtector, error) {
	protector, ok := r.protectors[code+"@"+version]
	if !ok {
		return nil, errors.New("unsupported DEK protector")
	}
	return protector, nil
}

// Capabilities 返回已注册真实引擎的编码与版本，用于测试构建和本地 Worker 自检。
func (r *Registry) Capabilities() []string {
	result := make([]string, 0, len(r.protectors))
	for key := range r.protectors {
		result = append(result, key)
	}
	return result
}
