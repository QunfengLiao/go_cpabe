package handler

import (
	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// AuthHandler 负责认证相关 HTTP 请求的参数绑定和响应包装。
type AuthHandler struct {
	service *service.AuthService
}

// NewAuthHandler 创建认证 Handler。
func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// registerRequest 是注册接口请求体，Role 仅允许公开可注册角色。
type registerRequest struct {
	Email           string          `json:"email"`
	Password        string          `json:"password"`
	ConfirmPassword string          `json:"confirm_password"`
	Nickname        string          `json:"nickname"`
	Role            domain.UserRole `json:"role"`
}

// loginRequest 是登录接口请求体，同时兼容 camelCase 和 snake_case 租户编码字段。
type loginRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	TenantCode      string `json:"tenantCode"`
	TenantCodeSnake string `json:"tenant_code"`
	DeviceID        string `json:"device_id"`
	DeviceIDCamel   string `json:"deviceId"`
}

// refreshRequest 是刷新 access token 的请求体。
type refreshRequest struct {
	RefreshToken  string `json:"refresh_token"`
	DeviceID      string `json:"device_id"`
	DeviceIDCamel string `json:"deviceId"`
}

// logoutRequest 是退出登录的请求体，必须携带要失效的 refresh token。
type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Register 处理用户公开注册请求，成功后返回不含密码字段的用户信息。
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

// Login 处理用户登录请求，成功后返回 token、用户信息和租户上下文。
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
		DeviceID:   firstNonEmpty(req.DeviceID, req.DeviceIDCamel),
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
		"currentTenant":            result.Tenant.CurrentTenant,
		"tenantRoles":              result.Tenant.TenantRoles,
		"permissions":              result.Tenant.Permissions,
		"tenants":                  result.Tenant.Tenants,
		"platform_roles":           result.PlatformRoles,
	})
}

// firstNonEmpty 返回第一个非空字符串，用于兼容不同前端字段命名。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// Refresh 处理 Refresh Token 轮换请求，成功后返回新的 token 组合。
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
		DeviceID:     firstNonEmpty(req.DeviceID, req.DeviceIDCamel),
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

// Logout 处理退出登录请求，成功后删除服务端刷新会话。
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
