package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	cryptomodule "go-cpabe/backend/internal/crypto"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
)

// TestEncryptionServiceCreateIdempotentAndComplete 验证算法、公钥、不可变授权、幂等任务、上传和完成事务主链路。
func TestEncryptionServiceCreateIdempotentAndComplete(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, repositoryLayer, _, audit := newEncryptionServiceFixture(client)
	input := encryptionCreateInput()
	created, idempotent, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", input)
	if err != nil || idempotent {
		t.Fatalf("create failed: idempotent=%t err=%v", idempotent, err)
	}
	if created.Task.AuthorizationSnapshotSHA256 == "" || strings.Contains(string(created.Task.AuthorizationSnapshot), "public_key_pem") {
		t.Fatalf("unsafe authorization snapshot: %s", created.Task.AuthorizationSnapshot)
	}
	if created.Attempt.StartedAt.IsZero() || created.Attempt.UpdatedAt.IsZero() {
		t.Fatalf("attempt timestamps must be initialized before persistence: %+v", created.Attempt)
	}
	second, idempotent, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", input)
	if err != nil || !idempotent || second.Task.PublicID != created.Task.PublicID || repositoryLayer.createCalls != 2 {
		t.Fatalf("idempotency failed: %+v %t %v", second.Task, idempotent, err)
	}
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatal(err)
	}
	completeInput := encryptionCompleteInput(object.PublicID)
	completed, duplicate, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, completeInput)
	if err != nil || duplicate || completed.File.Status != domain.EncryptedFileAvailable {
		t.Fatalf("complete failed: %+v duplicate=%t err=%v", completed.File, duplicate, err)
	}
	if len(audit.events) < 3 {
		t.Fatalf("expected create/upload/complete audit events, got %d", len(audit.events))
	}
}

// TestEncryptionServiceCreatesAndCompletesMultipleRecipients 验证服务层按接收者数组冻结快照，
// 完成时必须提交多份 protected DEK，避免单接收者业务模型继续漏到主链路。
func TestEncryptionServiceCreatesAndCompletesMultipleRecipients(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, repositoryLayer, _, _ := newEncryptionServiceFixture(client)
	input := encryptionCreateInput()
	input.Authorization = map[string]any{"type": "RSA_RECIPIENTS", "recipients": []any{
		map[string]any{"user_id": uint64(7), "public_key_id": "owner-key"},
		map[string]any{"user_id": uint64(9), "public_key_id": "623e4567-e89b-42d3-a456-426614174000"},
	}}
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "multi-recipient-0001", input)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(created.Task.AuthorizationSnapshot), `"recipients"`) {
		t.Fatalf("multi recipient snapshot missing recipients: %s", created.Task.AuthorizationSnapshot)
	}
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatal(err)
	}
	completeInput := encryptionMultiRecipientCompleteInput(object.PublicID)
	if _, _, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, completeInput); err != nil {
		t.Fatal(err)
	}
	if len(repositoryLayer.lastCompletion.ProtectedKeys) != 2 {
		t.Fatalf("expected two protected keys in completion, got %+v", repositoryLayer.lastCompletion)
	}
}

// TestEncryptionServiceRejectsIncompleteOrDuplicateProtectedKeySet 验证完成请求必须与冻结接收者集合一一对应，不能少交或重复提交某人的 protected DEK。
func TestEncryptionServiceRejectsIncompleteOrDuplicateProtectedKeySet(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		mutate func([]CompleteProtectedKeyInput) []CompleteProtectedKeyInput
	}{
		{name: "missing", mutate: func(keys []CompleteProtectedKeyInput) []CompleteProtectedKeyInput { return keys[:1] }},
		{name: "duplicate", mutate: func(keys []CompleteProtectedKeyInput) []CompleteProtectedKeyInput {
			return []CompleteProtectedKeyInput{keys[0], keys[0]}
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			serviceLayer, _, _, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
			input := encryptionCreateInput()
			input.Authorization = map[string]any{"type": "RSA_RECIPIENTS", "recipients": []any{
				map[string]any{"user_id": uint64(7), "public_key_id": "owner-key"},
				map[string]any{"user_id": uint64(9), "public_key_id": "623e4567-e89b-42d3-a456-426614174000"},
			}}
			created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "set-check-"+scenario.name, input)
			if err != nil {
				t.Fatal(err)
			}
			object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
			if err != nil {
				t.Fatal(err)
			}
			completeInput := encryptionMultiRecipientCompleteInput(object.PublicID)
			completeInput.ProtectedKeys = scenario.mutate(completeInput.ProtectedKeys)
			if _, _, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, completeInput); err != response.ErrProtectedKeyInvalid {
				t.Fatalf("expected protected key set rejection, got %v", err)
			}
		})
	}
}

// TestEncryptionServiceRejectsDuplicateRecipients 验证同一接收者重复授权会在任务创建阶段被拒绝。
func TestEncryptionServiceRejectsDuplicateRecipients(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, _, _, _ := newEncryptionServiceFixture(client)
	input := encryptionCreateInput()
	input.Authorization = map[string]any{"type": "RSA_RECIPIENTS", "recipients": []any{
		map[string]any{"user_id": uint64(9), "public_key_id": "623e4567-e89b-42d3-a456-426614174000"},
		map[string]any{"user_id": uint64(9), "public_key_id": "623e4567-e89b-42d3-a456-426614174000"},
	}}
	if _, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "multi-recipient-0002", input); err != response.ErrProtectedKeyInvalid {
		t.Fatalf("duplicate recipient error=%v", err)
	}
}

// TestEncryptionServiceRejectsMissingOwnerRecipient 验证后端不能创建“拥有者自己没有 protected DEK”的任务；
// 否则文件列表只能诚实显示不可解密，后续也无法通过本地私钥恢复 DEK。
func TestEncryptionServiceRejectsMissingOwnerRecipient(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, _, _, _ := newEncryptionServiceFixture(client)
	input := encryptionCreateInput()
	input.Authorization = map[string]any{"type": "RSA_RECIPIENTS", "recipients": []any{
		map[string]any{"user_id": uint64(9), "public_key_id": "623e4567-e89b-42d3-a456-426614174000"},
	}}
	if _, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "missing-owner-0001", input); err != response.ErrEncryptionOwnerKeyRequired {
		t.Fatalf("missing owner recipient error=%v", err)
	}
}

// TestEncryptionServiceRejectsCrossRecipientAndChangedIdempotencyBody 验证跨接收者公钥和复用幂等键改变请求体均被拒绝。
func TestEncryptionServiceRejectsCrossRecipientAndChangedIdempotencyBody(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, _, _, _ := newEncryptionServiceFixture(client)
	input := encryptionCreateInput()
	input.Authorization["recipient_user_id"] = uint64(99)
	if _, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", input); err != response.ErrRSAKeyNotFound {
		t.Fatalf("cross recipient error=%v", err)
	}
	input = encryptionCreateInput()
	if _, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", input); err != nil {
		t.Fatal(err)
	}
	input.File.Name = "changed.txt"
	if _, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "1234567890abcdef", input); err != response.ErrEncryptionStateConflict {
		t.Fatalf("changed idempotency body error=%v", err)
	}
}

// TestRSAAdapterAllowsFrozenDisabledKey 验证禁用只阻止新任务，既有任务仍可按冻结版本完成绑定。
func TestRSAAdapterAllowsFrozenDisabledKey(t *testing.T) {
	key := domain.RSAPublicKey{ID: 21, PublicID: "623e4567-e89b-42d3-a456-426614174000", TenantID: 3, UserID: 9, Version: 1, FingerprintSHA256: strings.Repeat("c", 64), Status: "DISABLED"}
	adapter := NewRSAEncryptionAlgorithmAdapter(NewRSAKeyService(&rsaKeyRepositoryStub{keys: []domain.RSAPublicKey{key}}, NoopAuditRecorder{}))
	contextHash := strings.Repeat("d", 64)
	snapshot := map[string]any{"type": "RSA_RECIPIENT", "recipient_user_id": uint64(9), "rsa_public_key_id": key.PublicID, "public_key_version": uint32(1), "public_key_fingerprint_sha256": key.FingerprintSHA256}
	raw := map[string]any{"type": "RSA_RECIPIENT", "recipient_user_id": uint64(9), "rsa_public_key_id": key.PublicID, "public_key_fingerprint_sha256": key.FingerprintSHA256, "oaep_hash": "SHA-256", "oaep_label_sha256": contextHash}
	protected := cryptomodule.ProtectedKeyResult{ContextSHA256: contextHash}
	_, binding, err := adapter.ValidateCompletion(context.Background(), 3, snapshot, protected, raw)
	if err != nil || binding == nil {
		t.Fatalf("frozen disabled key should complete: binding=%T err=%v", binding, err)
	}
}

// encryptionCreateInput 构造首期 RSA 接收者任务输入。
func encryptionCreateInput() CreateEncryptionTaskInput {
	var input CreateEncryptionTaskInput
	input.File.Name, input.File.Size = "demo.txt", 6
	input.Algorithm.Code, input.Algorithm.Version = "RSA-OAEP-SHA256", "1"
	input.Authorization = map[string]any{"type": "RSA_RECIPIENT", "recipient_user_id": uint64(7), "rsa_public_key_id": "owner-key"}
	return input
}

// encryptionCompleteInput 构造算法无关受保护密钥与 RSA 专属绑定完成输入。
func encryptionCompleteInput(uploadID string) CompleteEncryptionInput {
	var input CompleteEncryptionInput
	contextHash := strings.Repeat("d", 64)
	input.UploadID = uploadID
	input.ContentEncryption.Algorithm, input.ContentEncryption.ContainerFormat, input.ContentEncryption.NoncePrefixBase64 = "AES-256-GCM", "GCPABE01", base64.StdEncoding.EncodeToString(make([]byte, 8))
	input.ContentEncryption.ChunkSize, input.ContentEncryption.ChunkCount, input.ContentEncryption.TagLength, input.ContentEncryption.AADVersion, input.ContentEncryption.ContextSHA256 = 4*1024*1024, 1, 16, "1", contextHash
	input.ProtectedKey.AlgorithmCode, input.ProtectedKey.AlgorithmVersion, input.ProtectedKey.Format, input.ProtectedKey.ValueBase64, input.ProtectedKey.ContextSHA256 = "RSA-OAEP-SHA256", "1", "RSA-OAEP-SHA256-RAW", base64.StdEncoding.EncodeToString(make([]byte, 384)), contextHash
	input.AdapterBinding = map[string]any{"type": "RSA_RECIPIENT", "recipient_user_id": uint64(7), "rsa_public_key_id": "owner-key", "public_key_fingerprint_sha256": strings.Repeat("b", 64), "oaep_hash": "SHA-256", "oaep_label_sha256": contextHash}
	input.Benchmark.PlaintextSize, input.Benchmark.CiphertextSize, input.Benchmark.AESEncryptMS, input.Benchmark.DEKProtectMS, input.Benchmark.UploadMS, input.Benchmark.ClientRuntime = 6, 6, 10, 2, 3, "LOCAL_GO_WORKER"
	return input
}

// encryptionMultiRecipientCompleteInput 构造两接收者完成输入，所有受保护密钥共享同一上下文摘要。
func encryptionMultiRecipientCompleteInput(uploadID string) CompleteEncryptionInput {
	input := encryptionCompleteInput(uploadID)
	contextHash := input.ContentEncryption.ContextSHA256
	input.ProtectedKeys = []CompleteProtectedKeyInput{
		{AlgorithmCode: "RSA-OAEP-SHA256", AlgorithmVersion: "1", Format: "RSA-OAEP-SHA256-RAW", ValueBase64: base64.StdEncoding.EncodeToString(make([]byte, 384)), ContextSHA256: contextHash, AdapterBinding: map[string]any{"type": "RSA_RECIPIENT", "recipient_user_id": uint64(7), "rsa_public_key_id": "owner-key", "public_key_fingerprint_sha256": strings.Repeat("b", 64), "oaep_hash": "SHA-256", "oaep_label_sha256": contextHash, "protect_duration_ms": int64(4)}},
		{AlgorithmCode: "RSA-OAEP-SHA256", AlgorithmVersion: "1", Format: "RSA-OAEP-SHA256-RAW", ValueBase64: base64.StdEncoding.EncodeToString(make([]byte, 384)), ContextSHA256: contextHash, AdapterBinding: map[string]any{"type": "RSA_RECIPIENT", "recipient_user_id": uint64(9), "rsa_public_key_id": "623e4567-e89b-42d3-a456-426614174000", "public_key_fingerprint_sha256": strings.Repeat("c", 64), "oaep_hash": "SHA-256", "oaep_label_sha256": contextHash, "protect_duration_ms": int64(5)}},
	}
	return input
}

// TestEncryptionServiceAuditFailureDoesNotBlockSuccess 验证审计失败降级后不阻断加密主链路。
func TestEncryptionServiceAuditFailureDoesNotBlockSuccess(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, _, _, audit := newEncryptionServiceFixture(client)
	audit.err = errors.New("audit down")
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "audit-degrade-0001", encryptionCreateInput())
	if err != nil {
		t.Fatalf("create should ignore audit failure: %v", err)
	}
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatalf("upload should ignore audit failure: %v", err)
	}
	if _, _, err := serviceLayer.Complete(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, encryptionCompleteInput(object.PublicID)); err != nil {
		t.Fatalf("complete should ignore audit failure: %v", err)
	}
}

// TestEncryptionServiceUploadReturnsExistingStaging 验证同一 attempt 相同摘要和大小的重复上传直接返回已有 STAGING 对象。
func TestEncryptionServiceUploadReturnsExistingStaging(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, repositoryLayer, storageLayer, _ := newEncryptionServiceFixture(client)
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "upload-idempotent-0001", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	repositoryLayer.findStagingObject = &domain.CiphertextObject{PublicID: "existing-upload", TenantID: 3, TaskAttemptID: created.Attempt.ID, ContainerFormat: "GCPABE01", CiphertextSize: 6, CiphertextSHA256: strings.Repeat("a", 64), Status: domain.CiphertextStaging}
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("A", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil {
		t.Fatalf("repeat upload should reuse staging object: %v", err)
	}
	if object.PublicID != "existing-upload" {
		t.Fatalf("unexpected reused upload id: %+v", object)
	}
	if storageLayer.stageCalls != 0 {
		t.Fatalf("idempotent upload must not write another staging file, calls=%d", storageLayer.stageCalls)
	}
}

// TestEncryptionServiceUploadRejectsConflictingStaging 验证同一 attempt 已有不同摘要的暂存对象时返回冲突，避免静默覆盖。
func TestEncryptionServiceUploadRejectsConflictingStaging(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	serviceLayer, repositoryLayer, _, _ := newEncryptionServiceFixture(client)
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "upload-conflict-0001", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	repositoryLayer.findStagingObject = &domain.CiphertextObject{PublicID: "existing-upload", TenantID: 3, TaskAttemptID: created.Attempt.ID, ContainerFormat: "GCPABE01", CiphertextSize: 6, CiphertextSHA256: strings.Repeat("b", 64), Status: domain.CiphertextStaging}
	if _, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher")); err != response.ErrEncryptionStateConflict {
		t.Fatalf("conflicting staging upload error=%v", err)
	}
}

// TestEncryptionServiceUploadRecoversConcurrentUniqueConflict 验证两个相同上传竞争唯一键时，落败请求删除自身临时对象并复用已登记对象。
func TestEncryptionServiceUploadRecoversConcurrentUniqueConflict(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, repositoryLayer, storageLayer, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "upload-race-000001", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	winner := &domain.CiphertextObject{PublicID: "winner-upload", TenantID: 3, TaskAttemptID: created.Attempt.ID, ContainerFormat: "GCPABE01", CiphertextSize: 6, CiphertextSHA256: strings.Repeat("a", 64), Status: domain.CiphertextStaging}
	repositoryLayer.saveStagingError = errors.New("duplicate task_attempt_id")
	repositoryLayer.stagingAfterError = winner
	object, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, strings.Repeat("a", 64), "GCPABE01", 6, strings.NewReader("cipher"))
	if err != nil || object.PublicID != winner.PublicID {
		t.Fatalf("concurrent retry should reuse winner: object=%+v err=%v", object, err)
	}
	if storageLayer.deleted != 1 {
		t.Fatalf("losing staging object must be deleted, deleted=%d", storageLayer.deleted)
	}
}

// TestEncryptionServiceUploadRejectsInvalidDigest 验证缺失或非 SHA-256 的摘要在读取请求体前被拒绝。
func TestEncryptionServiceUploadRejectsInvalidDigest(t *testing.T) {
	server := miniredis.RunT(t)
	serviceLayer, _, storageLayer, _ := newEncryptionServiceFixture(redis.NewClient(&redis.Options{Addr: server.Addr()}))
	created, _, err := serviceLayer.CreateTask(context.Background(), 3, 7, "upload-digest-0001", encryptionCreateInput())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := serviceLayer.UploadCiphertext(context.Background(), 3, 7, created.Task.PublicID, created.Attempt.PublicID, "not-sha256", "GCPABE01", 6, strings.NewReader("cipher")); err != response.ErrCiphertextHashMismatch {
		t.Fatalf("invalid digest error=%v", err)
	}
	if storageLayer.stageCalls != 0 {
		t.Fatalf("invalid digest must be rejected before storage write, calls=%d", storageLayer.stageCalls)
	}
}
