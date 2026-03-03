package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError 统一错误格式
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// ErrResp 返回统一错误 JSON
func ErrResp(c *gin.Context, status int, code, message string, hint ...string) {
	h := ""
	if len(hint) > 0 {
		h = hint[0]
	}
	c.JSON(status, gin.H{"error": APIError{Code: code, Message: message, Hint: h}})
}

// AuthMiddleware 校验 X-Admin-Token
func AuthMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		t := c.GetHeader("X-Admin-Token")
		if t == "" {
			t = c.Query("token")
		}
		if t != token {
			ErrResp(c, http.StatusUnauthorized, "UNAUTHORIZED", "无效的 Admin Token")
			c.Abort()
			return
		}
		c.Next()
	}
}
