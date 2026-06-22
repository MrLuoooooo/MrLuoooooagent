package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// McpHandler 管理 MCP 服务器配置的 REST 端点。
type McpHandler struct {
	store  *service.McpStore
	logger *zap.Logger
}

// NewMcpHandler —
func NewMcpHandler(store *service.McpStore, logger *zap.Logger) *McpHandler {
	return &McpHandler{store: store, logger: logger}
}

// ListServers 获取所有 MCP 服务器配置。
func (h *McpHandler) ListServers(c *gin.Context) {
	servers, err := h.store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}
	if servers == nil {
		servers = []config.MCPServer{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"enabled": h.store.IsEnabled(),
		"servers": servers,
	}})
}

// UpsertServer 创建或更新 MCP 服务器配置。
func (h *McpHandler) UpsertServer(c *gin.Context) {
	var req config.MCPServer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	if err := h.store.Upsert(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// RemoveServer 删除 MCP 服务器配置。
func (h *McpHandler) RemoveServer(c *gin.Context) {
	name := c.Param("name")
	if err := h.store.Remove(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// ToggleEnabled 切换 MCP 启用状态。
func (h *McpHandler) ToggleEnabled(c *gin.Context) {
	var req struct{ Enabled bool `json:"enabled"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	if err := h.store.SetEnabled(req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}
