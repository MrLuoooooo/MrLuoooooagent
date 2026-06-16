package api

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client 股票数据 API 客户端接口。
type Client interface {
	GetStockData(ctx context.Context, code string) (*StockData, error)
	GetKLineData(ctx context.Context, code string, period string, limit int) ([]KLineData, error)
	GetName() string
}

// BaseClient 共享 HTTP 客户端和日志器。
type BaseClient struct {
	client *http.Client
	logger *zap.Logger
}

// NewBaseClient —
func NewBaseClient(logger *zap.Logger) *BaseClient {
	return &BaseClient{
		client: &http.Client{Timeout: 10 * time.Second},
		logger: logger,
	}
}
