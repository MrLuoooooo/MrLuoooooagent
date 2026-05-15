package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/goagentpro/internal/model"
)

// UserContextKey is the key used to store authenticated user info in the request context.
const UserContextKey = "auth_user"

// Auth returns a Gin middleware that enforces token authentication.
//
// When cfg.Auth.APIKey is set (production mode):
//   - Valid token → user = "authenticated", continue
//   - Missing/invalid token → 401 with {"code":401,"message":"unauthorized"}
//
// When cfg.Auth.APIKey is empty (development mode):
//   - Any request passes through; user is tagged as "anonymous" or "authenticated"
//     for observability, but no request is blocked.
func Auth(cfg *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := "anonymous"
		auth := c.GetHeader("Authorization")

		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")

			if cfg.APIKey != "" {
				if token == cfg.APIKey {
					user = "authenticated"
				} else {
					// Production: reject mismatched keys.
					c.Set(UserContextKey, "invalid_token")
					c.AbortWithStatusJSON(http.StatusUnauthorized, model.APIEnvelope{
						Code:    401,
						Message: "unauthorized",
					})
					return
				}
			} else if token != "" {
				// Dev mode: accept any non-empty token.
				user = "authenticated"
			}
		} else if cfg.APIKey != "" {
			// Production: no Authorization header → 401.
			c.Set(UserContextKey, "anonymous")
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.APIEnvelope{
				Code:    401,
				Message: "unauthorized",
			})
			return
		}

		c.Set(UserContextKey, user)
		c.Next()
	}
}

// Config mirrors the authentication section of the application config.
type Config struct {
	APIKey string
}
