package crypto

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// TestAuthenticatedChunkContainerRoundTrip 验证多分块容器、RSA 保护 DEK 和逐块 AAD 可完整还原原文。
func TestAuthenticatedChunkContainerRoundTrip(t *testing.T) {
	plaintext := bytes.Repeat([]byte("gcpabe-authenticated-chunk"), 200000)
	directory := t.TempDir()
	sourcePath, outputPath := filepath.Join(directory, "plain.bin"), filepath.Join(directory, "cipher.part")
	if err := os.WriteFile(sourcePath, plaintext, 0o600); err != nil {
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
	engine, err := NewEngine(registry)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.EncryptFile(context.Background(), EncryptFileRequest{SourcePath: sourcePath, OutputPath: outputPath, TenantID: 1, OwnerUserID: 2, TaskID: "task", AttemptID: "attempt", FileID: "file", PlaintextSize: int64(len(plaintext)), AlgorithmCode: AlgorithmRSAOAEP256, AlgorithmVersion: AlgorithmVersion1, AuthorizationSnapshotHash: SHA256Hex(bytes.Repeat([]byte{1}, 32)), Authorization: Authorization{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": fingerprint, "recipient_user_id": uint64(3), "rsa_public_key_id": "key"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	recovered, firstSealed, aead, nonce, aad := decryptTestContainer(t, ciphertext, privatePEM, result.ProtectedKey.Value)
	if !bytes.Equal(recovered, plaintext) {
		t.Fatal("container round trip mismatch")
	}
	firstSealed[len(firstSealed)-1] ^= 1
	if _, err := aead.Open(nil, nonce, firstSealed, aad); err == nil {
		t.Fatal("modified GCM tag must fail")
	}
}

// decryptTestContainer 按公开容器协议解析测试密文并返回首块认证参数供篡改断言。
func decryptTestContainer(t *testing.T, container []byte, privatePEM string, protectedDEK []byte) ([]byte, []byte, cipher.AEAD, []byte, []byte) {
	t.Helper()
	if len(container) < 14 || string(container[:8]) != ContainerMagic || binary.BigEndian.Uint16(container[8:10]) != ContainerVersion {
		t.Fatal("invalid container prefix")
	}
	headerLength := int(binary.BigEndian.Uint32(container[10:14]))
	headerBytes := container[14 : 14+headerLength]
	var header ContainerHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		t.Fatal(err)
	}
	headerHash := sha256.Sum256(headerBytes)
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		t.Fatal("private key PEM missing")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	dek, err := rsa.DecryptOAEP(sha256.New(), nil, parsed.(*rsa.PrivateKey), protectedDEK, headerHash[:])
	if err != nil {
		t.Fatal(err)
	}
	aesBlock, err := aes.NewCipher(dek)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(aesBlock)
	if err != nil {
		t.Fatal(err)
	}
	noncePrefix, err := decodeBase64(header.NoncePrefixBase64)
	if err != nil {
		t.Fatal(err)
	}
	offset := 14 + headerLength
	var recovered []byte
	var firstSealed, firstNonce, firstAAD []byte
	for index := uint32(0); index < header.ChunkCount; index++ {
		if offset+8 > len(container) {
			t.Fatal("truncated chunk prefix")
		}
		actualIndex := binary.BigEndian.Uint32(container[offset : offset+4])
		plainLength := binary.BigEndian.Uint32(container[offset+4 : offset+8])
		offset += 8
		if actualIndex != index || offset+int(plainLength)+GCMTagSize > len(container) {
			t.Fatal("invalid chunk order or length")
		}
		sealed := append([]byte(nil), container[offset:offset+int(plainLength)+GCMTagSize]...)
		offset += len(sealed)
		nonce, _ := ChunkNonce(noncePrefix, index)
		aad, _ := ChunkAAD(headerHash[:], index, header.ChunkCount, plainLength)
		plain, err := aead.Open(nil, nonce, sealed, aad)
		if err != nil {
			t.Fatal(err)
		}
		recovered = append(recovered, plain...)
		if index == 0 {
			firstSealed, firstNonce, firstAAD = sealed, nonce, aad
		}
	}
	if offset != len(container) {
		t.Fatal("appended container bytes must be rejected")
	}
	return recovered, firstSealed, aead, firstNonce, firstAAD
}

// decodeBase64 解析容器 nonce 前缀，测试辅助函数仍保持错误显式返回。
func decodeBase64(value string) ([]byte, error) { return base64.StdEncoding.DecodeString(value) }
