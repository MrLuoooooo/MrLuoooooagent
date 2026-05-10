package main

import (
	"context"
	"fmt"

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

func startServer(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger, engine *gin.Engine) {
	gin.SetMode(cfg.Server.Mode)
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
			logger.Info("Starting server", zap.String("addr", addr))
			go func() {
				if err := engine.Run(addr); err != nil {
					logger.Error("Server failed", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Stopping server")
			return nil
		},
	})
}
