package repository

import (
	"math"
	"os"
	"strings"
	"testing"
)

// TestSanitizeAuditMetadata 验证动作级白名单保留合法标量，非法字段只产生已脱敏的最小事件。
func TestSanitizeAuditMetadata(t *testing.T) {
	taskID := "123e4567-e89b-42d3-a456-426614174000"
	attemptID := "223e4567-e89b-42d3-a456-426614174000"
	metadata, redacted := sanitizeAuditMetadata("encryption.progress", map[string]any{"task_id": taskID, "attempt_id": attemptID, "stage": "UPLOADING", "processed_bytes": int64(7)})
	if redacted || metadata["processed_bytes"] != int64(7) {
		t.Fatalf("合法进度元数据不应被清空: metadata=%+v redacted=%t", metadata, redacted)
	}
	metadata, redacted = sanitizeAuditMetadata("encryption.dek.protect", map[string]any{"task_id": taskID, "attempt_id": attemptID, "algorithm_code": "RSA-OAEP-SHA256", "algorithm_version": "1", "dek_protect_ms": int64(3), "recipient_count": 2, "protected_key_total_size": int64(768)})
	if redacted || metadata["recipient_count"] != 2 {
		t.Fatalf("合法 DEK 聚合指标不应被清空: metadata=%+v redacted=%t", metadata, redacted)
	}
	for _, value := range []map[string]any{{"private_key": "secret"}, {"access_token": "secret"}, {"source_path": "C:/secret"}, {"processed_bytes": -1}, {"processed_bytes": math.NaN()}} {
		metadata, redacted = sanitizeAuditMetadata("encryption.progress", value)
		if !redacted || len(metadata) != 0 {
			t.Fatalf("非法元数据必须整体清空且标记脱敏: input=%+v metadata=%+v redacted=%t", value, metadata, redacted)
		}
	}
}

// TestSanitizeAuditMetadataAllowsTenantMemberCreation 验证成员创建审计只接受角色摘要和创建标记等非敏感标量。
func TestSanitizeAuditMetadataAllowsTenantMemberCreation(t *testing.T) {
	metadata, redacted := sanitizeAuditMetadata("tenant_member.account_created", map[string]any{"roles": "DO,DU", "created_user": true})
	if redacted || metadata["roles"] != "DO,DU" || metadata["created_user"] != true {
		t.Fatalf("unexpected metadata=%+v redacted=%t", metadata, redacted)
	}
}

// TestSanitizeAuditMetadataRejectsWrongActionAndFormat 验证字段不能跨动作复用，摘要、UUID 和字符串长度必须符合约束。
func TestSanitizeAuditMetadataRejectsWrongActionAndFormat(t *testing.T) {
	cases := []map[string]any{
		{"file_id": "123e4567-e89b-42d3-a456-426614174000"},
		{"task_id": "not-a-uuid", "attempt_id": "223e4567-e89b-42d3-a456-426614174000", "stage": "UPLOADING", "processed_bytes": 1},
		{"task_id": "123e4567-e89b-42d3-a456-426614174000", "attempt_id": "223e4567-e89b-42d3-a456-426614174000", "ciphertext_size": 7, "ciphertext_sha256": strings.Repeat("A", 64)},
	}
	for _, input := range cases {
		metadata, redacted := sanitizeAuditMetadata("encryption.progress", input)
		if !redacted || len(metadata) != 0 {
			t.Fatalf("非法动作或格式必须被清空: input=%+v metadata=%+v", input, metadata)
		}
	}
}

// TestAuditQueryIsTenantScoped 验证审计查询始终要求可信 tenant_id。
func TestAuditQueryIsTenantScoped(t *testing.T) {
	content, err := os.ReadFile("audit_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `Where("tenant_id = ?", tenantID)`) {
		t.Fatal("audit query lost tenant scope")
	}
}
