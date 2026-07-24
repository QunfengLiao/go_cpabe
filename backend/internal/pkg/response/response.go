package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope 是所有 HTTP JSON 响应的统一外层结构。
type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

// OK 返回成功响应，data 可为对象、数组或 nil。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: "OK", Message: "success", Data: data, RequestID: requestID(c)})
}

// Created 返回资源创建成功响应，HTTP 状态码为 201。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Code: "OK", Message: "success", Data: data, RequestID: requestID(c)})
}

// Accepted 返回任务已受理响应；调用方应通过状态接口观察后台执行结果。
func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, Envelope{Code: "OK", Message: "accepted", Data: data, RequestID: requestID(c)})
}

// Fail 将业务错误或未知错误转换为统一响应结构。
func Fail(c *gin.Context, err error) {
	appErr := ErrInternal
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus, Envelope{Code: appErr.Code, Message: appErr.Message, Data: nil, RequestID: requestID(c)})
		return
	}
	c.JSON(ErrInternal.HTTPStatus, Envelope{Code: ErrInternal.Code, Message: ErrInternal.Message, Data: nil, RequestID: requestID(c)})
}

// requestID 从 gin.Context 中读取请求 ID，用于前端和日志关联。
func requestID(c *gin.Context) string {
	if v := c.GetHeader("X-Request-ID"); v != "" {
		return v
	}
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
