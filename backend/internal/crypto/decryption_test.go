package crypto

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEngineDecryptFileRestoresMultiChunkPlaintext 验证真实 RSA 私钥恢复 DEK 并逐块校验还原原文。
func TestEngineDecryptFileRestoresMultiChunkPlaintext(t *testing.T) {
	directory := t.TempDir()
	plaintext := bytes.Repeat([]byte("DU-local-decryption"), int(DefaultChunkSize/19)+1000)
	source, ciphertext, output := filepath.Join(directory, "source.bin"), filepath.Join(directory, "cipher.gcpabe"), filepath.Join(directory, "restored.bin")
	if err := os.WriteFile(source, plaintext, 0o600); err != nil {
		t.Fatal(err)
	}
	publicPEM, privatePEM, fingerprint, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	if err := registry.Register(RSAEngine{}); err != nil {
		t.Fatal(err)
	}
	engine, _ := NewEngine(registry)
	encrypted, err := engine.EncryptFile(context.Background(), EncryptFileRequest{SourcePath: source, OutputPath: ciphertext, TenantID: 3, OwnerUserID: 7, TaskID: "task", AttemptID: "attempt", FileID: "file-uuid", PlaintextSize: int64(len(plaintext)), AlgorithmCode: AlgorithmRSAOAEP256, AlgorithmVersion: AlgorithmVersion1, AuthorizationSnapshotHash: strings.Repeat("a", 64), Authorization: Authorization{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": fingerprint, "recipient_user_id": uint64(9), "rsa_public_key_id": "key"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.DecryptFile(context.Background(), DecryptFileRequest{CiphertextPath: ciphertext, OutputPath: output, PrivateKeyPEM: privatePEM, ProtectedKeyBase64: base64.StdEncoding.EncodeToString(encrypted.ProtectedKey.Value), ContextSHA256: encrypted.ContextSHA256, TenantID: 3, FileID: "file-uuid", RSAPublicKeyID: "key"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := os.ReadFile(output)
	if result.PlaintextSize != int64(len(plaintext)) || !bytes.Equal(restored, plaintext) {
		t.Fatalf("restored plaintext mismatch: result=%+v", result)
	}
}

// TestEngineDecryptFileRemovesOutputOnTamper 验证密文认证失败不会保留部分明文。
func TestEngineDecryptFileRemovesOutputOnTamper(t *testing.T) {
	directory := t.TempDir()
	output := filepath.Join(directory, "restored.bin")
	request, ciphertext := encryptionFixtureForTamper(t, directory)
	bytes, _ := os.ReadFile(ciphertext)
	bytes[len(bytes)-1] ^= 0xff
	if err := os.WriteFile(ciphertext, bytes, 0o600); err != nil {
		t.Fatal(err)
	}
	request.OutputPath = output
	engine, _ := NewEngine(NewRegistry())
	if _, err := engine.DecryptFile(context.Background(), request, nil); err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("partial plaintext remains: %v", err)
	}
}

// encryptionFixtureForTamper 创建单块真实 RSA+AES 密文和对应本地解密输入。
func encryptionFixtureForTamper(t *testing.T, directory string) (DecryptFileRequest, string) {
	t.Helper()
	source, ciphertext := filepath.Join(directory, "source.bin"), filepath.Join(directory, "cipher.gcpabe")
	_ = os.WriteFile(source, []byte("authenticated plaintext"), 0o600)
	publicPEM, privatePEM, fingerprint, _ := GenerateRSAKeyPair()
	registry := NewRegistry()
	_ = registry.Register(RSAEngine{})
	engine, _ := NewEngine(registry)
	result, err := engine.EncryptFile(context.Background(), EncryptFileRequest{SourcePath: source, OutputPath: ciphertext, TenantID: 3, OwnerUserID: 7, TaskID: "task", AttemptID: "attempt", FileID: "file-uuid", PlaintextSize: 23, AlgorithmCode: AlgorithmRSAOAEP256, AlgorithmVersion: AlgorithmVersion1, AuthorizationSnapshotHash: strings.Repeat("a", 64), Authorization: Authorization{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": fingerprint}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return DecryptFileRequest{CiphertextPath: ciphertext, PrivateKeyPEM: privatePEM, ProtectedKeyBase64: base64.StdEncoding.EncodeToString(result.ProtectedKey.Value), ContextSHA256: result.ContextSHA256, TenantID: 3, FileID: "file-uuid"}, ciphertext
}
