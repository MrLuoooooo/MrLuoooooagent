package server

import (
	"os"
	"path/filepath"
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

// @title           GoAgent Pro API
// @version         4.1.2
// @description     AI Agent 后端系统。让大模型安全操控本地开发环境——读写文件、执行命令、联网搜索、查询股票、检索知识库。
// @termsOfService  https://github.com/MrLuoooooo/MrLuoooooagent

// @contact.name   MrLuoooooo
// @contact.url    https://github.com/MrLuoooooo/MrLuoooooagent

// @license.name   MIT
// @license.url    https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1

// NewRouter 搭 Gin 引擎，挂所有路由和中间件。
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
	fbH *handler.FeedbackHandler,
	stockH *handler.StockHandler,
	mcpH *handler.McpHandler,
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
		v1.DELETE("/conversations", convH.DeleteAllConversations)
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
		v1.POST("/feedback", fbH.SubmitFeedback)
		v1.GET("/stock/kline", stockH.KLine)
		v1.GET("/stock/realtime", stockH.Realtime)
		v1.GET("/stock/search", stockH.Search)
		v1.GET("/stock/watchlist", stockH.GetWatchlist)
		v1.POST("/stock/watchlist", stockH.AddWatchlist)
		v1.DELETE("/stock/watchlist/:code", stockH.RemoveWatchlist)
		v1.GET("/mcp/servers", mcpH.ListServers)
		v1.POST("/mcp/servers", mcpH.UpsertServer)
		v1.DELETE("/mcp/servers/:name", mcpH.RemoveServer)
		v1.POST("/mcp/enabled", mcpH.ToggleEnabled)
		v1.POST("/mcp/import", mcpH.ImportZip)
		v1.POST("/mcp/servers/:name/connect", mcpH.Connect)
	}

	// SPA 静态文件服务（web/dist）
	distPath := filepath.Join(findProjectRoot(), "web", "dist")
	if _, err := os.Stat(distPath); err == nil {
		engine.Static("/assets", filepath.Join(distPath, "assets"))
		engine.StaticFile("/vite.svg", filepath.Join(distPath, "vite.svg"))
		engine.NoRoute(func(c *gin.Context) {
			c.File(filepath.Join(distPath, "index.html"))
		})
	}

	return engine
}

// findProjectRoot walks upward from the working directory to locate go.mod.
func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
