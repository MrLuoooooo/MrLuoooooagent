// @title           GoAgentPro API
// @version         4.1.2
// @description     智能 Agent 对话系统 — RAG 检索增强生成、工具调用、多轮对话
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization

package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/scheduler"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/server"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func main() {
	app := fx.New(
		server.Module,
		fx.Invoke(startServer),
	)
	app.Run()
}

func startServer(
	lc fx.Lifecycle,
	cfg *config.Config,
	logger *zap.Logger,
	engine *gin.Engine,
	cronScheduler *scheduler.CronScheduler,
	rateLimiter *middleware.RateLimiter,
) {
	gin.SetMode(cfg.Server.Mode)

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: engine,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting server", zap.String("addr", srv.Addr))
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("Server failed", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Shutting down server")

			cronScheduler.Stop()
			rateLimiter.Stop()

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				logger.Error("HTTP server shutdown error", zap.Error(err))
			}

			logger.Info("Server stopped")
			return nil
		},
	})
}
