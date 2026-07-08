package handler

import (
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

// UserHandler 负责当前用户资料和头像上传相关 HTTP 请求。
type UserHandler struct {
	service       *service.UserService
	maxAvatarSize int64
}

// NewUserHandler 创建用户 Handler，并在未配置头像大小时使用默认限制。
func NewUserHandler(service *service.UserService, maxAvatarSize int64) *UserHandler {
	if maxAvatarSize <= 0 {
		maxAvatarSize = 2 * 1024 * 1024
	}
	return &UserHandler{service: service, maxAvatarSize: maxAvatarSize}
}

// updateMeRequest 是当前用户资料编辑请求体，不能包含角色或状态字段。
type updateMeRequest struct {
	Nickname string `json:"nickname"`
	Bio      string `json:"bio"`
	Birthday string `json:"birthday"`
}

// Me 返回当前登录用户资料，用户 ID 来自认证中间件写入的 gin.Context。
func (h *UserHandler) Me(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	user, err := h.service.Me(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"user": user})
}

// UpdateMe 处理当前用户资料更新请求，只允许修改昵称、简介和生日。
func (h *UserHandler) UpdateMe(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest)
		return
	}
	user, err := h.service.UpdateProfile(c.Request.Context(), userID, service.UpdateProfileInput{
		Nickname: req.Nickname,
		Bio:      req.Bio,
		Birthday: req.Birthday,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"user": user})
}

// UploadAvatar 处理头像上传请求，先在 HTTP 层校验文件大小和类型再交给 Service。
func (h *UserHandler) UploadAvatar(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		response.Fail(c, response.ErrAccessTokenInvalid)
		return
	}
	file, err := c.FormFile("avatar")
	if err != nil {
		response.Fail(c, response.ErrAvatarEmpty)
		return
	}
	if file.Size <= 0 {
		response.Fail(c, response.ErrAvatarEmpty)
		return
	}
	if file.Size > h.maxAvatarSize {
		response.Fail(c, response.ErrAvatarTooLarge)
		return
	}
	if !allowedAvatar(file.Filename, file.Header.Get("Content-Type")) {
		response.Fail(c, response.ErrAvatarUnsupportedType)
		return
	}
	src, err := file.Open()
	if err != nil {
		response.Fail(c, response.ErrAvatarEmpty)
		return
	}
	defer src.Close()
	avatarURL, err := h.service.UploadAvatar(c.Request.Context(), userID, service.AvatarUploadInput{
		Filename:    file.Filename,
		ContentType: file.Header.Get("Content-Type"),
		Reader:      src,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"avatar_url": avatarURL})
}

// currentUserID 从 gin.Context 读取认证中间件写入的用户 ID。
func currentUserID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(middleware.ContextUserID)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}

// allowedAvatar 根据文件扩展名和 Content-Type 判断头像类型是否可接受。
func allowedAvatar(filename, contentType string) bool {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "jpg", "jpeg", "png", "webp":
	default:
		return false
	}
	if contentType == "" {
		return true
	}
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg", "image/png", "image/webp", "application/octet-stream":
		return true
	default:
		// 部分桌面端上传会给出更细的 image/* 类型；最终仍以扩展名白名单和大小限制兜底。
		return strings.HasPrefix(strings.ToLower(contentType), "image/")
	}
}
