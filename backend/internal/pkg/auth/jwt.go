package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-cpabe/backend/internal/domain"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("token invalid")
	ErrTokenType    = errors.New("token type mismatch")
)

type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

func NewManager(secret string, accessTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL}
}

func (m *Manager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *Manager) GenerateAccessToken(userID uint64, role domain.UserRole) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)
	jti, err := randomHex(16)
	if err != nil {
		return "", time.Time{}, err
	}
	claims := Claims{
		UserID:    userID,
		Role:      role,
		TokenType: string(domain.TokenTypeAccess),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	value, err := token.SignedString(m.secret)
	return value, expiresAt, err
}

func (m *Manager) ParseAccessToken(tokenValue string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrTokenInvalid
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	if token == nil || !token.Valid {
		return nil, ErrTokenInvalid
	}
	if claims.TokenType != string(domain.TokenTypeAccess) {
		return nil, ErrTokenType
	}
	return claims, nil
}
