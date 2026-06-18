package handler

import (
	"net/http"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock"
	stockdb "github.com/MrLuoooooo/MrLuoooooagent/internal/stock/db"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StockHandler 股票数据 REST 端点。
// 纯调度层，依赖 Collector + StockDB。
type StockHandler struct {
	collector *stock.Collector
	db        stockdb.StockDB
	logger    *zap.Logger
}

// NewStockHandler —
func NewStockHandler(collector *stock.Collector, db stockdb.StockDB, logger *zap.Logger) *StockHandler {
	return &StockHandler{collector: collector, db: db, logger: logger}
}

// KLine 返回 K 线数据。
// GET /api/v1/stock/kline?code=sh600519&period=day&limit=120
func (h *StockHandler) KLine(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		code = "sh600519"
	}
	period := c.DefaultQuery("period", "day")
	limit := 120
	if v := c.Query("limit"); v != "" {
		c.BindQuery(&struct{ Limit int }{})
	}

	data, err := h.collector.FetchKLine(c.Request.Context(), code, period, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	if len(data) > limit {
		data = data[len(data)-limit:]
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data})
}

// Realtime 返回实时行情。
// GET /api/v1/stock/realtime?code=sh600519
func (h *StockHandler) Realtime(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "code required"})
		return
	}
	data, err := h.collector.FetchRealtime(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data})
}

// Search 搜索股票。
// GET /api/v1/stock/search?keyword=茅台
func (h *StockHandler) Search(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": []interface{}{}})
		return
	}
	results, err := h.db.Search(keyword, 10)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

// Watchlist 自选股管理（内存存储）。
var watchlist []string

// GetWatchlist 获取自选股列表。
func (h *StockHandler) GetWatchlist(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": watchlist})
}

// AddWatchlist 添加自选股。
func (h *StockHandler) AddWatchlist(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Code == "" {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "invalid code"})
		return
	}
	for _, w := range watchlist {
		if w == req.Code {
			c.JSON(http.StatusOK, gin.H{"code": 0})
			return
		}
	}
	watchlist = append(watchlist, req.Code)
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// RemoveWatchlist 删除自选股。
func (h *StockHandler) RemoveWatchlist(c *gin.Context) {
	code := c.Param("code")
	for i, w := range watchlist {
		if w == code {
			watchlist = append(watchlist[:i], watchlist[i+1:]...)
			break
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}
