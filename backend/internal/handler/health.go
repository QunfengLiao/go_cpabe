package handler

import (
	"net/http"

	"go-cpabe/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	service *service.HealthService
}

func NewHealthHandler(service *service.HealthService) *HealthHandler {
	return &HealthHandler{service: service}
}

func (h *HealthHandler) Get(c *gin.Context) {
	health := h.service.Check(c.Request.Context())
	c.JSON(http.StatusOK, health)
}
