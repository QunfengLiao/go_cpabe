package handler

import (
	"net/http"

	"go-cpabe/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// HealthHandler 负责健康检查 HTTP 接口。
type HealthHandler struct {
	service *service.HealthService
}

// NewHealthHandler 创建健康检查 Handler。
func NewHealthHandler(service *service.HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

// Get 处理健康检查请求，返回应用和依赖的聚合健康状态。
func (h *HealthHandler) Get(c *gin.Context) {
	health := h.service.Check(c.Request.Context())
	c.JSON(http.StatusOK, health)
}
