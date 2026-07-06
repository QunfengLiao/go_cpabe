package handler

import (
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go-cpabe/backend/internal/middleware"
	"go-cpabe/backend/internal/pkg/response"
	"go-cpabe/backend/internal/service"
)

type UserHandler struct {
	service       *service.UserService
	maxAvatarSize int64
}

func NewUserHandler(service *service.UserService, maxAvatarSize int64) *UserHandler {
	if maxAvatarSize <= 0 {
		maxAvatarSize = 2 * 1024 * 1024
	}
	return &UserHandler{service: service, maxAvatarSize: maxAvatarSize}
}

type updateMeRequest struct {
	Nickname string `json:"nickname"`
	Bio      string `json:"bio"`
	Birthday string `json:"birthday"`
}

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

func currentUserID(c *gin.Context) (uint64, bool) {
	value, ok := c.Get(middleware.ContextUserID)
	if !ok {
		return 0, false
	}
	id, ok := value.(uint64)
	return id, ok
}

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
		return strings.HasPrefix(strings.ToLower(contentType), "image/")
	}
}
