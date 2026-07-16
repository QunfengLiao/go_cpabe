package crypto

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"testing"
)

// TestRSAEngineRoundTrip 验证首个 RSA 适配器使用上下文 label 保护 32 字节 DEK。
func TestRSAEngineRoundTrip(t *testing.T) {
	publicPEM, privatePEM, fingerprint, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	contextHash := sha256.Sum256([]byte("context"))
	dek := bytes.Repeat([]byte{0x42}, 32)
	result, err := (RSAEngine{}).Protect(context.Background(), dek, Authorization{Type: "RSA_RECIPIENT", Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": fingerprint, "recipient_user_id": uint64(7), "rsa_public_key_id": "key-id"}}, contextHash[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Value) != 384 || result.Format != "RSA-OAEP-SHA256-RAW" {
		t.Fatalf("unexpected protected key: %+v", result)
	}
	block, _ := pem.Decode([]byte(privatePEM))
	if block == nil {
		t.Fatal("private PEM missing")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := parsed.(*rsa.PrivateKey)
	recovered, err := rsa.DecryptOAEP(sha256.New(), nil, privateKey, result.Value, contextHash[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(recovered, dek) {
		t.Fatal("DEK round trip mismatch")
	}
	if _, err := rsa.DecryptOAEP(sha256.New(), nil, privateKey, result.Value, []byte("wrong")); err == nil {
		t.Fatal("wrong OAEP label must fail")
	}
}

// TestRSAEngineRejectsFingerprintMismatch 验证公钥材料不能与授权快照指纹脱钩。
func TestRSAEngineRejectsFingerprintMismatch(t *testing.T) {
	publicPEM, _, _, err := GenerateRSAKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	_, err = (RSAEngine{}).Protect(context.Background(), make([]byte, 32), Authorization{Parameters: map[string]any{"public_key_pem": publicPEM, "public_key_fingerprint_sha256": "wrong"}}, make([]byte, sha256.Size))
	if err == nil {
		t.Fatal("expected fingerprint mismatch")
	}
}
