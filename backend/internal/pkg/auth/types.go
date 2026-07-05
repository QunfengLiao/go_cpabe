package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-cpabe/backend/internal/domain"
)

type Claims struct {
	UserID    uint64          `json:"user_id"`
	Role      domain.UserRole `json:"role"`
	TokenType string          `json:"token_type"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresIn  int64     `json:"access_token_expires_in"`
	RefreshToken          string    `json:"refresh_token,omitempty"`
	RefreshTokenExpiresIn int64     `json:"refresh_token_expires_in"`
	TokenType             string    `json:"token_type"`
	AccessExpiresAt       time.Time `json:"-"`
	RefreshExpiresAt      time.Time `json:"-"`
}

type RefreshSession struct {
	UserID           uint64          `json:"user_id"`
	Role             domain.UserRole `json:"role"`
	SessionID        string          `json:"session_id"`
	RefreshTokenHash string          `json:"refresh_token_hash"`
	IssuedAt         time.Time       `json:"issued_at"`
	ExpiresAt        time.Time       `json:"expires_at"`
	UserAgent        string          `json:"user_agent,omitempty"`
	ClientIP         string          `json:"client_ip,omitempty"`
}

type RefreshTokenParts struct {
	TokenID string
	Secret  string
}
