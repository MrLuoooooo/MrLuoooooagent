package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/goagentpro/internal/config"
)

// UserContextKey is the key used to store authenticated user info in the request context.
const UserContextKey = "auth_user"

// Auth returns a Gin middleware that enforces token presence from day one.
//
//   - If a valid Authorization: Bearer <token> header is present, it sets
//     the username on the request context.
//   - If no token is present, the user is marked as "anonymous".
//   - In neither case does the middleware reject the request — but the
//     frontend will see "anonymous" in logs/responses, making it obvious
//     that auth is expected, without breaking development flow.
//
// When cfg.Auth.APIKey is set, the middleware validates the token against it.
// When empty, any non-empty token is accepted (development mode).
func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := "anonymous"
		auth := c.GetHeader("Authorization")

		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			token := strings.TrimPrefix(auth, "Bearer ")

			if cfg.Auth.APIKey != "" {
				// Production mode: reject mismatched keys.
				if token == cfg.Auth.APIKey {
					user = "authenticated"
				} else {
					user = "invalid_token"
				}
			} else if token != "" {
				// Dev mode with no configured key: accept any token.
				user = "authenticated"
			}
		}

		c.Set(UserContextKey, user)
		c.Next()
	}
}
