package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
)

// RSAEngine 使用 RSA-OAEP-SHA256 保护 DEK，是通用 CryptoEngine 的首个实现。
type RSAEngine struct{}

// Code 返回稳定算法编码。
func (RSAEngine) Code() string { return AlgorithmRSAOAEP256 }

// Version 返回 RSA 参数协议版本。
func (RSAEngine) Version() string { return AlgorithmVersion1 }

// Protect 校验授权中的 SPKI 公钥并用上下文摘要作为 OAEP label 保护 DEK。
func (RSAEngine) Protect(_ context.Context, dek []byte, authorization Authorization, contextHash []byte) (ProtectedKeyResult, error) {
	if len(dek) != 32 {
		return ProtectedKeyResult{}, errors.New("DEK must be 32 bytes")
	}
	publicPEM, _ := authorization.Parameters["public_key_pem"].(string)
	recipientID := authorization.Parameters["recipient_user_id"]
	publicKeyID := authorization.Parameters["rsa_public_key_id"]
	publicKey, fingerprint, err := ParseRSAPublicKey(publicPEM)
	if err != nil {
		return ProtectedKeyResult{}, err
	}
	if publicKey.N.BitLen() != 3072 {
		return ProtectedKeyResult{}, errors.New("RSA public key must be 3072 bits")
	}
	if expected, _ := authorization.Parameters["public_key_fingerprint_sha256"].(string); expected != "" && expected != fingerprint {
		return ProtectedKeyResult{}, errors.New("RSA public key fingerprint mismatch")
	}
	protected, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, dek, contextHash)
	if err != nil {
		return ProtectedKeyResult{}, err
	}
	return ProtectedKeyResult{
		AlgorithmCode:    AlgorithmRSAOAEP256,
		AlgorithmVersion: AlgorithmVersion1,
		Format:           "RSA-OAEP-SHA256-RAW",
		Value:            protected,
		ContextSHA256:    hex.EncodeToString(contextHash),
		Binding: map[string]any{
			"type":                          "RSA_RECIPIENT",
			"recipient_user_id":             recipientID,
			"rsa_public_key_id":             publicKeyID,
			"public_key_fingerprint_sha256": fingerprint,
			"oaep_hash":                     "SHA-256",
			"oaep_label_sha256":             hex.EncodeToString(contextHash),
		},
	}, nil
}

// ParseRSAPublicKey 解析 SPKI PEM 公钥并返回规范 DER 指纹。
func ParseRSAPublicKey(publicPEM string) (*rsa.PublicKey, string, error) {
	block, _ := pem.Decode([]byte(publicPEM))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, "", errors.New("invalid SPKI public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, "", err
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, "", errors.New("public key is not RSA")
	}
	fingerprint := sha256.Sum256(block.Bytes)
	return publicKey, hex.EncodeToString(fingerprint[:]), nil
}

// GenerateRSAKeyPair 生成 3072 位 RSA 密钥，并以 SPKI/PKCS8 PEM 返回给本地密钥保存边界。
func GenerateRSAKeyPair() (publicPEM, privatePEM, fingerprint string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return "", "", "", err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", "", err
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", "", err
	}
	sum := sha256.Sum256(publicDER)
	publicPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}))
	privatePEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))
	return publicPEM, privatePEM, hex.EncodeToString(sum[:]), nil
}
