package cache

import (
	"sync"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock/api"
	"go.uber.org/zap"
)

// MemoryCache 内存缓存，1 小时 TTL 过期自动清理。
type MemoryCache struct {
	stockData map[string]*cacheItem
	klineData map[string][]api.KLineData
	mu        sync.RWMutex
	expiry    time.Duration
	logger    *zap.Logger
}

type cacheItem struct {
	data      *api.StockData
	expiresAt time.Time
}

// NewMemoryCache —
func NewMemoryCache(logger *zap.Logger) *MemoryCache {
	c := &MemoryCache{
		stockData: make(map[string]*cacheItem),
		klineData: make(map[string][]api.KLineData),
		expiry:    1 * time.Hour,
		logger:    logger,
	}
	c.startCleanup(10 * time.Minute)
	return c
}

// GetStockData 取缓存行情，过期返回 false。
func (c *MemoryCache) GetStockData(code string) (*api.StockData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.stockData[code]
	if !ok {
		return nil, false
	}
	if time.Now().After(item.expiresAt) {
		return nil, false
	}
	return item.data, true
}

// SetStockData 写缓存行情。
func (c *MemoryCache) SetStockData(data *api.StockData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stockData[data.Code] = &cacheItem{data: data, expiresAt: time.Now().Add(c.expiry)}
}

// GetKLineData 取缓存 K 线。
func (c *MemoryCache) GetKLineData(code, period string) ([]api.KLineData, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, ok := c.klineData[code+":"+period]
	return data, ok
}

// SetKLineData 写缓存 K 线。
func (c *MemoryCache) SetKLineData(code, period string, data []api.KLineData) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.klineData[code+":"+period] = data
}

func (c *MemoryCache) startCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			c.cleanup()
		}
	}()
}

func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	count := 0
	for code, item := range c.stockData {
		if now.After(item.expiresAt) {
			delete(c.stockData, code)
			count++
		}
	}
	if count > 0 {
		c.logger.Debug("cache cleanup", zap.Int("removed", count))
	}
}
