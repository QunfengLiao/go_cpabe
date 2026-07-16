package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectcrypto "go-cpabe/backend/internal/crypto"
)

// TestExecuteEncryptFileFrames 验证 Worker 发送真实进度和无明文 DEK、私钥、路径的最终结果帧。
func TestExecuteEncryptFileFrames(t *testing.T) {
	directory := t.TempDir()
	sourcePath, outputPath := filepath.Join(directory, "plain.bin"), filepath.Join(directory, "cipher.part")
	payload := bytes.Repeat([]byte("worker"), 1024)
	if err := os.WriteFile(sourcePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	publicPEM, _, fingerprint, err := projectcrypto.GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	request := workerRequest{Operation: "encrypt_file", Encrypt: &projectcrypto.EncryptFileRequest{SourcePath: sourcePath, OutputPath: outputPath, TenantID: 1, OwnerUserID: 2, TaskID: "task", AttemptID: "attempt", FileID: "file", PlaintextSize: int64(len(payload)), AlgorithmCode: projectcrypto.AlgorithmRSAOAEP256, AlgorithmVersion: projectcrypto.AlgorithmVersion1, AuthorizationSnapshotHash: strings.Repeat("a", 64), Authorization: projectcrypto.Authorization{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": fingerprint, "recipient_user_id": uint64(3), "rsa_public_key_id": "key"}}}}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if err := execute(context.Background(), writer, request); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(bytes.NewReader(output.Bytes()))
	foundProgress, foundResult := false, false
	for {
		payload, err := readFrame(reader, 2*1024*1024)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var frame workerFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			t.Fatal(err)
		}
		foundProgress = foundProgress || frame.Type == "progress"
		if frame.Type == "result" {
			foundResult = true
			serialized := strings.ToLower(string(payload))
			if strings.Contains(serialized, "private_key") || strings.Contains(serialized, "source_path") || strings.Contains(serialized, "output_path") || strings.Contains(serialized, `"dek"`) {
				t.Fatalf("sensitive result frame: %s", serialized)
			}
		}
	}
	if !foundProgress || !foundResult {
		t.Fatalf("missing frames progress=%t result=%t", foundProgress, foundResult)
	}
}

// TestExecuteEncryptFileFramesMultipleProtectedKeys 验证 Worker 多接收者结果帧返回多份
// Base64 protected DEK，且不把明文 DEK 暴露给 Electron 主进程。
func TestExecuteEncryptFileFramesMultipleProtectedKeys(t *testing.T) {
	directory := t.TempDir()
	sourcePath, outputPath := filepath.Join(directory, "plain.bin"), filepath.Join(directory, "cipher.part")
	payload := bytes.Repeat([]byte("worker-multi"), 512)
	if err := os.WriteFile(sourcePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	firstPublic, _, firstFingerprint, err := projectcrypto.GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	secondPublic, _, secondFingerprint, err := projectcrypto.GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	request := workerRequest{Operation: "encrypt_file", Encrypt: &projectcrypto.EncryptFileRequest{
		SourcePath: sourcePath, OutputPath: outputPath, TenantID: 1, OwnerUserID: 2, TaskID: "task", AttemptID: "attempt", FileID: "file",
		PlaintextSize: int64(len(payload)), AlgorithmCode: projectcrypto.AlgorithmRSAOAEP256, AlgorithmVersion: projectcrypto.AlgorithmVersion1,
		AuthorizationSnapshotHash: strings.Repeat("a", 64),
		Authorizations: []projectcrypto.Authorization{
			{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": firstPublic, "public_key_fingerprint_sha256": firstFingerprint, "recipient_user_id": uint64(2), "rsa_public_key_id": "owner-key"}},
			{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": secondPublic, "public_key_fingerprint_sha256": secondFingerprint, "recipient_user_id": uint64(3), "rsa_public_key_id": "recipient-key"}},
		},
	}}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if err := execute(context.Background(), writer, request); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(bytes.NewReader(output.Bytes()))
	for {
		payload, err := readFrame(reader, 2*1024*1024)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var frame workerFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			t.Fatal(err)
		}
		if frame.Type != "result" {
			continue
		}
		if len(frame.Result.ProtectedKeysBase64) != 2 {
			t.Fatalf("expected two protected keys, got %+v", frame.Result)
		}
		if frame.Result.ProtectedKeys[0].Value != nil || frame.Result.ProtectedKeys[1].Value != nil {
			t.Fatal("protected key bytes must only be returned through base64 envelopes")
		}
		return
	}
	t.Fatal("missing result frame")
}

// TestReadFrameRejectsOversize 验证长度前缀不能触发无界内存分配。
func TestReadFrameRejectsOversize(t *testing.T) {
	header := []byte{0, 32, 0, 1}
	if _, err := readFrame(bytes.NewReader(header), 1024); err == nil {
		t.Fatal("oversized frame must fail")
	}
}
