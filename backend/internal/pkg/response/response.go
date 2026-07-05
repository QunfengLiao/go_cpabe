package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Envelope{Code: "OK", Message: "success", Data: data, RequestID: requestID(c)})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Envelope{Code: "OK", Message: "success", Data: data, RequestID: requestID(c)})
}

func Fail(c *gin.Context, err error) {
	appErr := ErrInternal
	if errors.As(err, &appErr) {
		c.JSON(appErr.HTTPStatus, Envelope{Code: appErr.Code, Message: appErr.Message, Data: nil, RequestID: requestID(c)})
		return
	}
	c.JSON(ErrInternal.HTTPStatus, Envelope{Code: ErrInternal.Code, Message: ErrInternal.Message, Data: nil, RequestID: requestID(c)})
}

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
