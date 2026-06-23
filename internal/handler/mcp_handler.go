package handler

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	mcp_connector "github.com/MrLuoooooo/MrLuoooooagent/internal/component/mcp"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/tool"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// McpHandler 管理 MCP 服务器配置的 REST 端点。
type McpHandler struct {
	store     *service.McpStore
	connector *mcp_connector.Connector
	logger    *zap.Logger
}

// NewMcpHandler —
func NewMcpHandler(store *service.McpStore, connector *mcp_connector.Connector, logger *zap.Logger) *McpHandler {
	return &McpHandler{store: store, connector: connector, logger: logger}
}

// ListServers 获取所有 MCP 服务器配置。
func (h *McpHandler) ListServers(c *gin.Context) {
	servers, err := h.store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
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

// UpsertServer 创建或更新 MCP 服务器配置。保存后自动尝试连接。
func (h *McpHandler) UpsertServer(c *gin.Context) {
	var req config.MCPServer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "message": err.Error()})
		return
	}
	if err := h.store.Upsert(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}
	// 自动连接
	result, _ := h.connector.ConnectOne(c.Request.Context(), req)
	resp := gin.H{"code": 0}
	if result != nil {
		if result.Error != "" {
			resp["connected"] = false
			resp["error"] = result.Error
		} else {
			resp["connected"] = true
			resp["tool_count"] = len(result.Tools)
		}
	}
	c.JSON(http.StatusOK, resp)
}

// RemoveServer 删除 MCP 服务器配置。
func (h *McpHandler) RemoveServer(c *gin.Context) {
	name := c.Param("name")
	if err := h.store.Remove(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// ImportZip 接收 ZIP 文件上传，解压→检测项目类型→自动配置→连接。
func (h *McpHandler) ImportZip(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		name = "mcp-import"
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "请上传 ZIP 文件"})
		return
	}
	defer file.Close()

	// 解压到唯一临时目录，用完清理
	tmpDir, err := os.MkdirTemp("", "goagent-mcp-")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, header.Filename)
	f, err := os.Create(zipPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}
	if _, err := io.Copy(f, file); err != nil {
		f.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}
	f.Close()

	// 解压
	if err := unzip(zipPath, tmpDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "解压失败: " + err.Error()})
		return
	}

	// 检测项目类型并生成配置
	srv := detectProject(tmpDir, name)
	if srv.Transport == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "未识别项目类型——需要 package.json / requirements.txt / go.mod 之一"})
		return
	}

	if err := h.store.Upsert(srv); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}

	// 自动连接
	result, connErr := h.connector.ConnectOne(c.Request.Context(), srv)

	// 注册工具
	toolCount := 0
	if connErr == nil && result != nil {
		for _, t := range result.Tools {
			if err := tool.Register(t); err != nil {
				h.logger.Warn("mcp import: register tool failed", zap.Error(err))
			} else {
				toolCount++
			}
		}
	}

	resp := gin.H{
		"code":     0,
		"server":   srv,
		"connected": connErr == nil,
		"tool_count": toolCount,
	}
	if connErr != nil {
		resp["error"] = connErr.Error()
	}
	c.JSON(http.StatusOK, resp)
}

// Connect 手动重新连接指定服务器。
func (h *McpHandler) Connect(c *gin.Context) {
	name := c.Param("name")
	servers, err := h.store.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}
	var srv config.MCPServer
	found := false
	for _, s := range servers {
		if s.Name == name {
			srv = s
			found = true
			break
		}
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "message": "server not found"})
		return
	}

	result, err := h.connector.ConnectOne(c.Request.Context(), srv)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "connected": false, "error": err.Error()})
		return
	}

	toolCount := 0
	for _, t := range result.Tools {
		tool.Register(t)
		toolCount++
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "connected": true, "tool_count": toolCount, "tools": result.Tools})
}

// detectProject 扫描目录，根据项目文件推断运行方式。
func detectProject(dir, name string) config.MCPServer {
	// Node.js 项目
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return config.MCPServer{
			Name:      name,
			Transport: "stdio",
			Command:   "npx",
			Args:      []string{"tsx", "."},
		}
	}
	// Python 项目
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return config.MCPServer{
			Name:      name,
			Transport: "stdio",
			Command:   "python",
			Args:      []string{"-m", "mcp_server"},
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return config.MCPServer{
			Name:      name,
			Transport: "stdio",
			Command:   "python",
			Args:      []string{"-m", "server"},
		}
	}
	// Go 项目
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return config.MCPServer{
			Name:      name,
			Transport: "stdio",
			Command:   "go",
			Args:      []string{"run", "."},
		}
	}
	// 找 main.py / server.py / index.js 兜底
	for _, f := range []string{"main.py", "server.py", "index.js", "app.js", "index.ts", "server.ts", "main.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			cmd, args := guessCmd(f)
			return config.MCPServer{
				Name:      name,
				Transport: "stdio",
				Command:   cmd,
				Args:      args,
			}
		}
	}
	return config.MCPServer{}
}

func guessCmd(entry string) (string, []string) {
	switch {
	case strings.HasSuffix(entry, ".py"):
		return "python", []string{entry}
	case strings.HasSuffix(entry, ".js"), strings.HasSuffix(entry, ".mjs"):
		return "node", []string{entry}
	case strings.HasSuffix(entry, ".ts"):
		return "npx", []string{"tsx", entry}
	case strings.HasSuffix(entry, ".go"):
		return "go", []string{"run", entry}
	default:
		return entry, nil
	}
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path := filepath.Join(dest, f.Name)
		// 防路径穿越
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
			continue
		}
		os.MkdirAll(filepath.Dir(path), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(path)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(w, rc)
		rc.Close()
		w.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
