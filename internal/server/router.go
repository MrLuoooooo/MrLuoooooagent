package server

import (
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/handler"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/server/middleware"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/server/version"
	_ "github.com/MrLuoooooo/MrLuoooooagent/cmd/server/docs"
	"go.uber.org/zap"
)

// NewRouter creates and configures the Gin engine with all routes and middleware.
func NewRouter(
	cfg *config.Config,
	logger *zap.Logger,
	chatH *handler.ChatHandler,
	convH *handler.ConversationHandler,
	docH *handler.DocumentHandler,
	batchH *handler.BatchHandler,
	approvalH *handler.ApprovalHandler,
	modelH *handler.ModelHandler,
	skillH *handler.SkillHandler,
	wsH *handler.WorkspaceHandler,
	rl *middleware.RateLimiter,
) *gin.Engine {
	engine := gin.New()
	engine.MaxMultipartMemory = 8 << 20

	engine.Use(gin.Recovery())
	engine.Use(middleware.CORS(cfg.Server.CORSOrigins))

	if cfg.Server.RateLimitRPS > 0 {
		engine.Use(rl.Middleware())
	}

	engine.Use(middleware.Auth(&middleware.Config{APIKey: cfg.Auth.APIKey}))
	engine.Use(middleware.Logger(logger))

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"version":   version.AppVersion,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
		v1.POST("/batch", batchH.HandleBatch)
		v1.GET("/approvals/pending", approvalH.ListPending)
		v1.GET("/approvals", approvalH.ListAll)
		v1.POST("/approvals/:approval_id/decide", approvalH.Decide)
		v1.GET("/models", modelH.ListAvailable)
		v1.POST("/models/switch", modelH.SwitchModel)
		v1.POST("/models", modelH.AddCustomModel)
		v1.DELETE("/models/:name", modelH.RemoveCustomModel)
		v1.GET("/skills", skillH.List)
		v1.POST("/skills", skillH.Upsert)
		v1.DELETE("/skills/:name", skillH.Remove)
		v1.GET("/workspace", wsH.GetCurrent)
		v1.POST("/workspace", wsH.SetCurrent)
		v1.GET("/workspace/drives", wsH.ListDrives)
		v1.GET("/workspace/dir", wsH.ListDir)
		v1.GET("/workspace/tree", wsH.ListTree)
	}

	return engine
}
