package stock

import (
	"context"
	"fmt"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock/api"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock/cache"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock/storage"
	"go.uber.org/zap"
)

// Collector 三层数据获取：内存缓存(1h TTL) → 双源 API → 文件存储降级。
type Collector struct {
	clients []api.Client
	cache   *cache.MemoryCache
	store   *storage.FileStore
	logger  *zap.Logger
}

// NewCollector —
func NewCollector(clients []api.Client, cache *cache.MemoryCache, store *storage.FileStore, logger *zap.Logger) *Collector {
	return &Collector{clients: clients, cache: cache, store: store, logger: logger}
}

// FetchRealtime 获取实时行情：cache → API → local file。
func (c *Collector) FetchRealtime(ctx context.Context, code string) (*api.StockData, error) {
	// L1: 内存缓存（1h TTL）
	if data, ok := c.cache.GetStockData(code); ok {
		return data, nil
	}

	// L2: 双源 API 降级
	for _, cl := range c.clients {
		data, err := cl.GetStockData(ctx, code)
		if err == nil {
			c.cache.SetStockData(data)
			_ = c.store.SaveStockData(ctx, data)
			return data, nil
		}
		c.logger.Warn("stock api failed", zap.String("source", cl.GetName()), zap.String("code", code), zap.Error(err))
	}

	// L3: 本地文件
	data, err := c.store.GetStockData(ctx, code)
	if err == nil {
		c.cache.SetStockData(data)
		return data, nil
	}
	return nil, fmt.Errorf("all sources failed for %s: %w", code, err)
}

// FetchKLine 获取 K 线：cache → API → local file。
func (c *Collector) FetchKLine(ctx context.Context, code, period string, limit int) ([]api.KLineData, error) {
	// L1: 内存缓存
	if data, ok := c.cache.GetKLineData(code, period); ok {
		if limit > 0 && len(data) > limit {
			return data[len(data)-limit:], nil
		}
		return data, nil
	}

	// L2: 双源 API 降级
	for _, cl := range c.clients {
		data, err := cl.GetKLineData(ctx, code, period, limit)
		if err == nil {
			c.cache.SetKLineData(code, period, data)
			_ = c.store.SaveKLineData(ctx, code, period, data)
			return data, nil
		}
		c.logger.Warn("stock kline api failed", zap.String("source", cl.GetName()), zap.String("code", code), zap.Error(err))
	}

	// L3: 本地文件
	data, err := c.store.GetKLineData(ctx, code, period, limit)
	if err == nil {
		c.cache.SetKLineData(code, period, data)
		return data, nil
	}
	return nil, fmt.Errorf("all sources failed for %s kline %s: %w", code, period, err)
}
