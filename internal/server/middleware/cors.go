package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 加跨域头，允许前端访问。
// allowedOrigins 非空时用第一个 origin，否则用 *。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowOrigin := "*"
	if len(allowedOrigins) > 0 && allowedOrigins[0] != "*" {
		allowOrigin = allowedOrigins[0]
	}

	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
