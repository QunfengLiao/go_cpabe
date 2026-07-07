package service

import (
	"context"
	"errors"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/pkg/validator"
	"go-cpabe/backend/internal/repository"
)

type RegisterInput struct {
	Email           string
	Password        string
	ConfirmPassword string
	Nickname        string
	Role            domain.UserRole
}

type LoginInput struct {
	Email      string
	Password   string
	TenantCode string
	UserAgent  string
	ClientIP   string
}

type RefreshInput struct {
	RefreshToken string
	UserAgent    string
	ClientIP     string
}

type AuthService struct {
	users      repository.UserRepository
	manager    *auth.Manager
	store      auth.TokenStore
	refreshTTL time.Duration
	tenants    *TenantService
}

func NewAuthService(users repository.UserRepository, manager *auth.Manager, store auth.TokenStore, refreshTTL time.Duration, tenants ...*TenantService) *AuthService {
	svc := &AuthService{users: users, manager: manager, store: store, refreshTTL: refreshTTL}
	if len(tenants) > 0 {
		svc.tenants = tenants[0]
	}
	return svc
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (domain.UserDTO, error) {
	if !validator.ValidEmail(input.Email) {
		return domain.UserDTO{}, response.ErrInvalidEmail
	}
	if input.Password == "" || input.ConfirmPassword == "" {
		return domain.UserDTO{}, response.ErrBadRequest
	}
	if input.Password != input.ConfirmPassword {
		return domain.UserDTO{}, response.ErrPasswordConfirmMismatch
	}
	if !validator.ValidNickname(input.Nickname) {
		return domain.UserDTO{}, response.ErrBadRequest
	}
	if input.Role == domain.RoleAdmin {
		return domain.UserDTO{}, response.ErrAdminRegisterForbidden
	}
	if !input.Role.PublicRegistrable() {
		return domain.UserDTO{}, response.ErrInvalidRole
	}
	if _, err := s.users.FindByEmail(ctx, input.Email); err == nil {
		return domain.UserDTO{}, response.ErrEmailAlreadyExists
	} else if !errors.Is(err, repository.ErrUserNotFound) {
		return domain.UserDTO{}, err
	}
	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return domain.UserDTO{}, err
	}
	user := &domain.User{
		Email:        input.Email,
		PasswordHash: passwordHash,
		Nickname:     input.Nickname,
		Role:         input.Role,
		Status:       domain.StatusActive,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return domain.UserDTO{}, err
	}
	if s.tenants != nil {
		if err := s.tenants.EnsureUserInDefaultTenant(ctx, user.ID, user.Role); err != nil {
			return domain.UserDTO{}, err
		}
	}
	return domain.ToUserDTO(*user, false), nil
}

type LoginResult struct {
	TokenPair auth.TokenPair
	User      domain.UserDTO
	Tenant    domain.TenantContextDTO
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	user, err := s.users.FindByEmail(ctx, input.Email)
	if err != nil {
		return LoginResult{}, response.ErrInvalidCredentials
	}
	if user.Status == domain.StatusDisabled {
		return LoginResult{}, response.ErrUserDisabled
	}
	if !auth.CheckPassword(input.Password, user.PasswordHash) {
		return LoginResult{}, response.ErrInvalidCredentials
	}
	result := LoginResult{User: domain.ToUserDTO(*user, false)}
	if s.tenants != nil {
		tenantContext, err := s.tenants.TenantContextForUserByCode(ctx, user.ID, input.TenantCode)
		if err != nil {
			return LoginResult{}, err
		}
		result.Tenant = tenantContext
	}
	pair, err := s.issueTokenPair(ctx, user.ID, user.Role, "", input.UserAgent, input.ClientIP)
	if err != nil {
		return LoginResult{}, response.ErrRedisWriteFailed
	}
	result.TokenPair = pair
	return result, nil
}

func (s *AuthService) Refresh(ctx context.Context, input RefreshInput) (auth.TokenPair, error) {
	parts, err := auth.ParseRefreshToken(input.RefreshToken)
	if err != nil {
		return auth.TokenPair{}, response.ErrRefreshTokenInvalid
	}
	session, err := s.store.Get(ctx, parts.TokenID)
	if err != nil {
		return auth.TokenPair{}, response.ErrRefreshSessionNotFound
	}
	if session.RefreshTokenHash != auth.HashRefreshToken(input.RefreshToken) {
		return auth.TokenPair{}, response.ErrRefreshTokenMismatch
	}
	user, err := s.users.FindByID(ctx, session.UserID)
	if err != nil {
		return auth.TokenPair{}, response.ErrRefreshSessionNotFound
	}
	if user.Status == domain.StatusDisabled {
		return auth.TokenPair{}, response.ErrUserDisabled
	}
	return s.issueTokenPair(ctx, user.ID, user.Role, parts.TokenID, input.UserAgent, input.ClientIP)
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	parts, err := auth.ParseRefreshToken(refreshToken)
	if err != nil {
		return response.ErrRefreshTokenInvalid
	}
	session, err := s.store.Get(ctx, parts.TokenID)
	if err != nil {
		return response.ErrRefreshSessionNotFound
	}
	if session.RefreshTokenHash != auth.HashRefreshToken(refreshToken) {
		return response.ErrRefreshTokenMismatch
	}
	return s.store.Delete(ctx, parts.TokenID)
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID uint64, role domain.UserRole, oldTokenID string, userAgent string, clientIP string) (auth.TokenPair, error) {
	access, accessExpiresAt, err := s.manager.GenerateAccessToken(userID, role)
	if err != nil {
		return auth.TokenPair{}, err
	}
	tokenID, sessionID, refreshToken, err := auth.GenerateRefreshToken()
	if err != nil {
		return auth.TokenPair{}, err
	}
	now := time.Now().UTC()
	refreshExpiresAt := now.Add(s.refreshTTL)
	session := auth.RefreshSession{
		UserID:           userID,
		Role:             role,
		SessionID:        sessionID,
		RefreshTokenHash: auth.HashRefreshToken(refreshToken),
		IssuedAt:         now,
		ExpiresAt:        refreshExpiresAt,
		UserAgent:        userAgent,
		ClientIP:         clientIP,
	}
	if oldTokenID != "" {
		err = s.store.Rotate(ctx, oldTokenID, tokenID, session, s.refreshTTL)
	} else {
		err = s.store.Save(ctx, tokenID, session, s.refreshTTL)
	}
	if err != nil {
		return auth.TokenPair{}, err
	}
	return auth.TokenPair{
		AccessToken:           access,
		AccessTokenExpiresIn:  int64(s.manager.AccessTTL().Seconds()),
		RefreshToken:          refreshToken,
		RefreshTokenExpiresIn: int64(s.refreshTTL.Seconds()),
		TokenType:             "Bearer",
		AccessExpiresAt:       accessExpiresAt,
		RefreshExpiresAt:      refreshExpiresAt,
	}, nil
}
