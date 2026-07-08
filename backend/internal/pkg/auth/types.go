package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-cpabe/backend/internal/domain"
)

// Claims 是 access token 中携带的短期身份声明。
type Claims struct {
	UserID    uint64          `json:"user_id"`
	Role      domain.UserRole `json:"role"`
	TokenType string          `json:"token_type"`
	jwt.RegisteredClaims
}

// TokenPair 是登录或刷新成功后返回给客户端的一组访问凭证。
type TokenPair struct {
	AccessToken           string    `json:"access_token"`
	AccessTokenExpiresIn  int64     `json:"access_token_expires_in"`
	RefreshToken          string    `json:"refresh_token,omitempty"`
	RefreshTokenExpiresIn int64     `json:"refresh_token_expires_in"`
	TokenType             string    `json:"token_type"`
	AccessExpiresAt       time.Time `json:"-"`
	RefreshExpiresAt      time.Time `json:"-"`
}

// RefreshSession 是服务端保存的刷新会话，只存 Refresh Token 哈希，不存明文 token。
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

// RefreshTokenParts 是解析 refresh token 后得到的定位 ID 和客户端 secret。
type RefreshTokenParts struct {
	TokenID string
	Secret  string
}
