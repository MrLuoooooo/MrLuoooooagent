package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

// UserContextKey is the key used to store authenticated user info in the request context.
const UserContextKey = "auth_user"

// Auth 根据配置拦截未认证请求，开发模式下放行。
//
// cfg.Auth.APIKey 有值时（生产模式）：
//   - 有效 token → user = "authenticated"，继续
//   - 缺失/无效 token → 401 {"code":401,"message":"unauthorized"}
//
// cfg.Auth.APIKey 为空时（开发模式）：
//   - 任何请求都放行，user 标记为 "anonymous" 或 "authenticated"，不做拦截。
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
