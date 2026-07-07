package handler

import (
	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

type registerRequest struct {
	Email           string          `json:"email"`
	Password        string          `json:"password"`
	ConfirmPassword string          `json:"confirm_password"`
	Nickname        string          `json:"nickname"`
	Role            domain.UserRole `json:"role"`
}

type loginRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	TenantCode      string `json:"tenantCode"`
	TenantCodeSnake string `json:"tenant_code"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	user, err := h.service.Register(c.Request.Context(), service.RegisterInput{
		Email:           req.Email,
		Password:        req.Password,
		ConfirmPassword: req.ConfirmPassword,
		Nickname:        req.Nickname,
		Role:            req.Role,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, gin.H{"user": user})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	result, err := h.service.Login(c.Request.Context(), service.LoginInput{
		Email:      req.Email,
		Password:   req.Password,
		TenantCode: firstNonEmpty(req.TenantCode, req.TenantCodeSnake),
		UserAgent:  c.GetHeader("User-Agent"),
		ClientIP:   c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"access_token":             result.TokenPair.AccessToken,
		"access_token_expires_in":  result.TokenPair.AccessTokenExpiresIn,
		"refresh_token":            result.TokenPair.RefreshToken,
		"refresh_token_expires_in": result.TokenPair.RefreshTokenExpiresIn,
		"token_type":               result.TokenPair.TokenType,
		"user":                     result.User,
		"current_tenant_id":        result.Tenant.CurrentTenantID,
		"current_tenant_code":      result.Tenant.CurrentTenantCode,
		"tenants":                  result.Tenant.Tenants,
		"platform_roles":           result.PlatformRoles,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	if req.RefreshToken == "" {
		response.Fail(c, response.ErrRefreshTokenMissing)
		return
	}
	pair, err := h.service.Refresh(c.Request.Context(), service.RefreshInput{
		RefreshToken: req.RefreshToken,
		UserAgent:    c.GetHeader("User-Agent"),
		ClientIP:     c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{
		"access_token":             pair.AccessToken,
		"access_token_expires_in":  pair.AccessTokenExpiresIn,
		"refresh_token":            pair.RefreshToken,
		"refresh_token_expires_in": pair.RefreshTokenExpiresIn,
		"token_type":               pair.TokenType,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req logoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	if req.RefreshToken == "" {
		response.Fail(c, response.ErrRefreshTokenMissing)
		return
	}
	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"logged_out": true})
}
