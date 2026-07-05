package router

import (
	"net/http"

	"go-cpabe/backend/internal/config"
	"go-cpabe/backend/internal/handler"
	"go-cpabe/backend/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config     config.Config
	MySQL      *gorm.DB
	MySQLError error
	Redis      *redis.Client
	RedisError error
}

func New(deps Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), corsMiddleware())

	healthService := service.NewHealthService(deps.Config, deps.MySQL, deps.MySQLError, deps.Redis, deps.RedisError)
	healthHandler := handler.NewHealthHandler(healthService)

	r.GET("/health", healthHandler.Get)

	return r
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
