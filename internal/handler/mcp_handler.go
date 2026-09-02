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

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "请上传 ZIP 文件"})
		return
	}
	defer file.Close()

	// 解压到永久目录 (项目 ID 作为子目录名)
	projectDir := filepath.Join("/var/lib/goagent-projects", name)
	_ = os.RemoveAll(projectDir) // 覆盖已有同名项目
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "error": err.Error()})
		return
	}
	zipPath := filepath.Join(projectDir, "src.zip")
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
	if err := unzip(zipPath, projectDir); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "error": "解压失败: " + err.Error()})
		return
	}

	// 检测项目类型并生成配置
	srv := detectProject(projectDir, name)
	if srv.Transport == "" {
		// 未识别 manifest — 仍然接受上传，让 Agent 后续改造
		srv.Name = name // detectProject 未命中时返回零值结构体，漏 Name 会让列表出现空名条目
		srv.Transport = "agent"
		srv.Command = "true"
		srv.Args = []string{}
	}
	// stdio 入口文件改为绝对路径（客户端从 /app 启动子进程）
	if srv.Transport == "stdio" && len(srv.Args) > 0 {
		srv.Args[0] = filepath.Join(projectDir, srv.Args[0])
	}
	srv.URL = projectDir

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
		"code":      0,
		"server":    srv,
		"connected": connErr == nil,
		"tool_count": toolCount,
	}
	if srv.Transport == "agent" {
		// agent-managed：无连接语义，导入即成功（前端按 mode 渲染文案）
		resp["mode"] = "agent-managed"
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
	// 优先级 1：明确的入口文件（main.py / server.py / index.js 等）
	entries := []struct {
		file, cmd string
		args      []string
	}{
		{"main.py", "python3", []string{"main.py"}},
		{"server.py", "python3", []string{"server.py"}},
		{"index.js", "node", []string{"index.js"}},
		{"app.js", "node", []string{"app.js"}},
		{"index.ts", "npx", []string{"tsx", "index.ts"}},
		{"server.ts", "npx", []string{"tsx", "server.ts"}},
		{"main.go", "go", []string{"run", "main.go"}},
	}
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(dir, e.file)); err == nil {
			return config.MCPServer{
				Name: name, Transport: "stdio",
				Command: e.cmd, Args: e.args,
			}
		}
	}

	// 优先级 2：包管理器 (无明确入口文件时用框架推断)
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return config.MCPServer{
			Name: name, Transport: "stdio",
			Command: "npx", Args: []string{"tsx", "."},
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
		return config.MCPServer{
			Name: name, Transport: "stdio",
			Command: "python3", Args: []string{"-m", "mcp_server"},
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		return config.MCPServer{
			Name: name, Transport: "stdio",
			Command: "python3", Args: []string{"-m", "server"},
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return config.MCPServer{
			Name: name, Transport: "stdio",
			Command: "go", Args: []string{"run", "."},
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
