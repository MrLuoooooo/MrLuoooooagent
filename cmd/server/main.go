package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/goagentpro/internal/config"
	server "github.com/yourusername/goagentpro/internal/server"
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
			logger.Info("Shutting down server gracefully...")

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
