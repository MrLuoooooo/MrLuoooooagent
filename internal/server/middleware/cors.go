package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware that adds CORS headers for frontend access.
// If allowedOrigins is non-empty, the first origin is used in Allow-Origin.
// When empty or the first entry is "*", Allow-Origin: * is used.
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
