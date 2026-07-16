package crypto

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// testDEKProtector 是仅测试注册的非产品适配器，不冒充任何 CP-ABE 实现。
type testDEKProtector struct{ authorization Authorization }

// Code 返回明确的测试算法编码。
func (p *testDEKProtector) Code() string { return "TEST-DEK-PROTECTOR" }

// Version 返回测试适配器版本。
func (p *testDEKProtector) Version() string { return "test-1" }

// Protect 捕获规范化授权并返回算法无关测试结果，不实现产品密码学。
func (p *testDEKProtector) Protect(_ context.Context, dek []byte, authorization Authorization, contextHash []byte) (ProtectedKeyResult, error) {
	p.authorization = authorization
	return ProtectedKeyResult{AlgorithmCode: p.Code(), AlgorithmVersion: p.Version(), Format: "TEST-ONLY", Value: append([]byte(nil), dek...), ContextSHA256: SHA256Hex(contextHash), Binding: map[string]any{"type": authorization.Type}}, nil
}

// TestNonProductProtectorReusesContentEngine 验证新 DEK 适配器复用同一 AES 容器协调器且不会进入产品目录。
func TestNonProductProtectorReusesContentEngine(t *testing.T) {
	directory := t.TempDir()
	source, output := filepath.Join(directory, "plain"), filepath.Join(directory, "cipher")
	if err := os.WriteFile(source, []byte("adapter contract"), 0o600); err != nil {
		t.Fatal(err)
	}
	protector := &testDEKProtector{}
	registry := NewRegistry()
	if err := registry.Register(protector); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(registry)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.EncryptFile(context.Background(), EncryptFileRequest{SourcePath: source, OutputPath: output, TenantID: 1, OwnerUserID: 2, TaskID: "task", AttemptID: "attempt", FileID: "file", PlaintextSize: int64(len("adapter contract")), AlgorithmCode: protector.Code(), AlgorithmVersion: protector.Version(), AuthorizationSnapshotHash: "snapshot", Authorization: Authorization{Type: "TEST_AUTHORIZATION", Parameters: map[string]any{"subject": "test"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProtectedKey.Format != "TEST-ONLY" || protector.authorization.Type != "TEST_AUTHORIZATION" || result.ChunkCount != 1 {
		t.Fatalf("contract result=%+v auth=%+v", result, protector.authorization)
	}
	for _, capability := range ProductCapabilities() {
		if capability.Code == protector.Code() {
			t.Fatal("test protector must not enter product catalog")
		}
	}
}

// TestEncryptFileProtectsOneDEKForMultipleRecipients 验证多接收者场景只生成一份密文容器，
// 同一个 DEK 会分别按接收者授权封装，避免为每个接收者重复加密和上传完整文件。
func TestEncryptFileProtectsOneDEKForMultipleRecipients(t *testing.T) {
	directory := t.TempDir()
	source, output := filepath.Join(directory, "plain"), filepath.Join(directory, "cipher")
	if err := os.WriteFile(source, []byte("multi recipient contract"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := &testDEKProtector{}
	registry := NewRegistry()
	if err := registry.Register(first); err != nil {
		t.Fatal(err)
	}
	engine, err := NewEngine(registry)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.EncryptFile(context.Background(), EncryptFileRequest{
		SourcePath:                source,
		OutputPath:                output,
		TenantID:                  1,
		OwnerUserID:               2,
		TaskID:                    "task",
		AttemptID:                 "attempt",
		FileID:                    "file",
		PlaintextSize:             int64(len("multi recipient contract")),
		AlgorithmCode:             first.Code(),
		AlgorithmVersion:          first.Version(),
		AuthorizationSnapshotHash: "snapshot",
		Authorizations: []Authorization{
			{Type: "TEST_AUTHORIZATION", Parameters: map[string]any{"recipient_user_id": uint64(2), "slot": "owner"}},
			{Type: "TEST_AUTHORIZATION", Parameters: map[string]any{"recipient_user_id": uint64(3), "slot": "recipient"}},
		},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ProtectedKeys) != 2 {
		t.Fatalf("expected two protected keys, got %+v", result.ProtectedKeys)
	}
	if base64.StdEncoding.EncodeToString(result.ProtectedKeys[0].Value) != base64.StdEncoding.EncodeToString(result.ProtectedKeys[1].Value) {
		t.Fatal("test protector should receive and return the same DEK for every recipient")
	}
	if result.ProtectedKey.Value != nil {
		t.Fatal("legacy single protected key must stay empty when multi-recipient output is used")
	}
	for index, protected := range result.ProtectedKeys {
		if _, ok := protected.Binding["protect_duration_ms"]; !ok {
			t.Fatalf("protected key %d missing recipient duration: %+v", index, protected.Binding)
		}
	}
}
