package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/repository"
	"go-cpabe/backend/internal/service"
)

// RSAKeyApplication 定义公钥 Handler 所需的最小业务能力，测试可验证响应绝不包含私钥字段。
type RSAKeyApplication interface {
	MyKeys(ctx context.Context, tenantID, actorUserID uint64) ([]domain.RSAPublicKey, error)
	RegisterMyKey(ctx context.Context, tenantID, actorUserID uint64, input service.RegisterRSAKeyInput) (domain.RSAPublicKey, bool, error)
	Recipients(ctx context.Context, tenantID, actorUserID uint64) ([]repository.RSARecipient, error)
	UpdateStatus(ctx context.Context, tenantID, actorUserID uint64, keyPublicID string, input service.UpdateRSAKeyStatusInput) (domain.RSAPublicKey, error)
}

// RSAKeyHandler 处理当前租户 RSA 公钥登记、接收者查询和管理员状态变更。
type RSAKeyHandler struct{ service RSAKeyApplication }

// NewRSAKeyHandler 创建 RSA 公钥 Handler。
func NewRSAKeyHandler(service RSAKeyApplication) *RSAKeyHandler {
	return &RSAKeyHandler{service: service}
}

// MyKeys 返回当前成员自己的公钥历史，响应结构不可能包含私钥字段。
func (h *RSAKeyHandler) MyKeys(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	keys, err := h.service.MyKeys(c.Request.Context(), tenantID, actorID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, keys)
}

// RegisterMyKey 登记客户端本地生成的 SPKI 公钥；重复指纹返回既有版本。
func (h *RSAKeyHandler) RegisterMyKey(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	var input service.RegisterRSAKeyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	key, idempotent, err := h.service.RegisterMyKey(c.Request.Context(), tenantID, actorID, input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if idempotent {
		response.OK(c, key)
		return
	}
	response.Created(c, key)
}

// Recipients 返回当前租户具有有效 RSA 公钥的接收者。
func (h *RSAKeyHandler) Recipients(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	items, err := h.service.Recipients(c.Request.Context(), tenantID, actorID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result := make([]gin.H, 0, len(items))
	for _, item := range items {
		result = append(result, gin.H{"user_id": item.UserID, "display_name": item.DisplayName, "available": len(item.Keys) > 0, "active_key_count": len(item.Keys)})
	}
	response.OK(c, result)
}

// RecipientKeys 返回指定接收者的有效公钥版本；用户 ID 只在当前租户结果中筛选。
func (h *RSAKeyHandler) RecipientKeys(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	userID, err := strconv.ParseUint(c.Param("userId"), 10, 64)
	if err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	items, err := h.service.Recipients(c.Request.Context(), tenantID, actorID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	for _, item := range items {
		if item.UserID == userID {
			response.OK(c, item.Keys)
			return
		}
	}
	response.OK(c, []any{})
}

// UpdateStatus 处理租户管理员禁用或撤销公钥版本。
func (h *RSAKeyHandler) UpdateStatus(c *gin.Context) {
	tenantID, actorID, ok := encryptionActor(c)
	if !ok {
		return
	}
	var input service.UpdateRSAKeyStatusInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	key, err := h.service.UpdateStatus(c.Request.Context(), tenantID, actorID, c.Param("keyId"), input)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, key)
}

// encryptionActor 从认证与租户中间件建立的可信上下文读取操作者，拒绝请求体自报身份。
func encryptionActor(c *gin.Context) (uint64, uint64, bool) {
	tenantID, hasTenant := middleware.CurrentTenantID(c)
	actorID, hasActor := currentUserID(c)
	if !hasTenant {
		response.Fail(c, response.ErrTenantIDMissing)
		return 0, 0, false
	}
	if !hasActor {
		c.AbortWithStatus(http.StatusUnauthorized)
		return 0, 0, false
	}
	return tenantID, actorID, true
}
