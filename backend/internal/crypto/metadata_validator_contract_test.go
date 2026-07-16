package crypto

import (
	"strings"
	"testing"
)

// TestMetadataValidatorDispatch 验证通用校验器精确按算法版本分派并隔离 RSA 绑定。
func TestMetadataValidatorDispatch(t *testing.T) {
	validator := NewMetadataValidator()
	valid := ProtectedKeyResult{AlgorithmCode: AlgorithmRSAOAEP256, AlgorithmVersion: AlgorithmVersion1, Format: "RSA-OAEP-SHA256-RAW", Value: make([]byte, 384), ContextSHA256: strings.Repeat("a", 64), Binding: map[string]any{"type": "RSA_RECIPIENT", "rsa_public_key_id": uint64(1)}}
	if err := validator.ValidateProtectedKey(valid); err != nil {
		t.Fatal(err)
	}
	valid.AlgorithmCode = "UNKNOWN"
	if err := validator.ValidateProtectedKey(valid); err == nil {
		t.Fatal("unknown algorithm must fail")
	}
}
