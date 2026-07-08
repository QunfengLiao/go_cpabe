package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// GenerateRefreshToken 生成 refresh token 的定位 ID、会话 ID 和返回给客户端的明文 token。
func GenerateRefreshToken() (tokenID, sessionID, token string, err error) {
	tokenID, err = randomHex(16)
	if err != nil {
		return "", "", "", err
	}
	sessionID, err = randomHex(16)
	if err != nil {
		return "", "", "", err
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", "", err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)
	// tokenID 用于定位 Redis 会话，secret 只返回给客户端；服务端仅保存整串 token 的哈希。
	return tokenID, sessionID, tokenID + "." + secret, nil
}

// ParseRefreshToken 拆分客户端传入的 refresh token，格式错误时返回解析错误。
func ParseRefreshToken(token string) (RefreshTokenParts, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return RefreshTokenParts{}, errors.New("invalid refresh token")
	}
	return RefreshTokenParts{TokenID: parts[0], Secret: parts[1]}, nil
}

// HashRefreshToken 对 refresh token 明文做 SHA-256 摘要，用于服务端会话存储和比对。
func HashRefreshToken(token string) string {
	// Refresh Token 具备长期登录能力，落库前必须哈希化，避免存储泄露后可直接重放。
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// randomHex 生成指定字节长度的安全随机十六进制字符串。
func randomHex(bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
