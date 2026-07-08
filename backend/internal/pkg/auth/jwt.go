package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go-cpabe/backend/internal/domain"
)

var (
	// ErrTokenExpired 表示 access token 已超过服务端允许的有效期。
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenInvalid 表示 token 签名、格式或声明无法通过认证校验。
	ErrTokenInvalid = errors.New("token invalid")
	// ErrTokenType 表示调用方把 refresh token 等非访问令牌用于受保护接口。
	ErrTokenType = errors.New("token type mismatch")
)

// Manager 负责 access token 的签发和解析，内部固定使用 HS256 签名算法。
type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewManager 创建 JWT 管理器，secret 用于 HS256 签名，accessTTL 控制短期 token 生命周期。
func NewManager(secret string, accessTTL time.Duration) *Manager {
	return &Manager{secret: []byte(secret), accessTTL: accessTTL}
}

// AccessTTL 返回 access token 的有效期，供响应字段计算 expires_in。
func (m *Manager) AccessTTL() time.Duration {
	return m.accessTTL
}

// GenerateAccessToken 为指定用户签发短期 access token，并返回过期时间。
func (m *Manager) GenerateAccessToken(userID uint64, role domain.UserRole) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)
	jti, err := randomHex(16)
	if err != nil {
		return "", time.Time{}, err
	}
	claims := Claims{
		UserID: userID,
		Role:   role,
		// access token 只承载短期身份信息；租户和平台权限仍由服务端查库确认，避免长期权限漂移。
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

// ParseAccessToken 校验 access token 签名、过期时间和 token 类型，成功后返回声明。
func (m *Manager) ParseAccessToken(tokenValue string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenValue, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			// 固定签名算法，避免攻击者把 token 头改成其他算法绕过校验。
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
		// refresh token 不能混用在 Authorization 头里，防止长期凭证被当作短期访问凭证。
		return nil, ErrTokenType
	}
	return claims, nil
}
