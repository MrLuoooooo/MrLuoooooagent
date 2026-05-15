package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/goagentpro/internal/config"
	"github.com/yourusername/goagentpro/internal/handler"
	"github.com/yourusername/goagentpro/internal/server/middleware"
	"go.uber.org/zap"
)

// NewRouter creates and configures the Gin engine with all routes and middleware.
func NewRouter(
	cfg *config.Config,
	logger *zap.Logger,
	chatH *handler.ChatHandler,
	convH *handler.ConversationHandler,
	docH *handler.DocumentHandler,
) *gin.Engine {
	engine := gin.New()

	// Global middleware
	engine.Use(gin.Recovery())
	engine.Use(middleware.CORS(cfg.Server.CORSOrigins))

	rl := middleware.NewRateLimiter(cfg.Server.RateLimitRPS, cfg.Server.RateLimitRPS*2)
	if cfg.Server.RateLimitRPS > 0 {
		engine.Use(rl.Middleware())
	}

	engine.Use(middleware.Auth(&middleware.Config{APIKey: cfg.Auth.APIKey}))
	engine.Use(middleware.Logger(logger))

	// Health
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// API v1
	v1 := engine.Group("/api/v1")
	{
		v1.POST("/chat", chatH.Chat)
		v1.POST("/conversations", convH.CreateConversation)
		v1.GET("/conversations", convH.ListConversations)
		v1.GET("/conversations/:conversation_id/messages", convH.GetMessages)
		v1.POST("/documents", docH.UploadDocument)
		v1.DELETE("/documents/:document_id", docH.DeleteDocument)
		v1.GET("/documents", docH.ListDocuments)
		v1.DELETE("/conversations/:conversation_id", convH.DeleteConversation)
	}

	return engine
}
