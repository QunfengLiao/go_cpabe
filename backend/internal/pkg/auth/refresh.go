package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

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
	return tokenID, sessionID, tokenID + "." + secret, nil
}

func ParseRefreshToken(token string) (RefreshTokenParts, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return RefreshTokenParts{}, errors.New("invalid refresh token")
	}
	return RefreshTokenParts{TokenID: parts[0], Secret: parts[1]}, nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(bytesLen int) (string, error) {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
