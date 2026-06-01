package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/tool"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"go.uber.org/zap"
)

// WorkspaceHandler 管 Agent 工作目录的切换和浏览。
type WorkspaceHandler struct {
	currentDir string
	logger     *zap.Logger
}

func toWin(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return p
	}
	if mnt := os.Getenv("HOST_MNT_PREFIX"); mnt != "" && strings.HasPrefix(p, mnt+"/") {
		rest := p[len(mnt)+1:]
		if len(rest) >= 2 && rest[1] == '/' {
			return strings.ToUpper(string(rest[0])) + ":\\" + strings.ReplaceAll(rest[2:], "/", "\\")
		}
	}
	if strings.HasPrefix(p, "/D/") {
		return "D:\\" + strings.TrimPrefix(p, "/D/")
	}
	return p
}

func toContainer(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		drive := strings.ToLower(string(p[0]))
		rest := strings.TrimLeft(p[3:], `/\`)
		if mnt := os.Getenv("HOST_MNT_PREFIX"); mnt != "" {
			return mnt + "/" + drive + "/" + rest
		}
		return "/" + strings.ToUpper(string(drive)) + "/" + rest
	}
	return p
}

// NewWorkspaceHandler 优先用配置里的工作目录，否则取当前目录。
func NewWorkspaceHandler(logger *zap.Logger, cfg config.ServerConfig) *WorkspaceHandler {
	// 优先使用配置中的工作目录
	if cfg.WorkspaceDir != "" {
		if info, err := os.Stat(cfg.WorkspaceDir); err == nil && info.IsDir() {
			wd := toWin(cfg.WorkspaceDir)
			tool.SetWorkspaceRoot(wd)
			logger.Info("workspace restored from config", zap.String("path", wd))
			return &WorkspaceHandler{currentDir: wd, logger: logger}
		}
		logger.Warn("configured workspace_dir not found, falling back", zap.String("path", cfg.WorkspaceDir))
	}
	// 自动检测候选目录
	candidates := []string{`D:\`, "/D/"}
	home, _ := os.UserHomeDir()
	if home != "" {
		candidates = append(candidates, home)
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			tool.SetWorkspaceRoot(dir)
			return &WorkspaceHandler{currentDir: toWin(dir), logger: logger}
		}
	}
	tool.SetWorkspaceRoot("/")
	return &WorkspaceHandler{currentDir: "/", logger: logger}
}


// FileNode represents a node in the directory tree.
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"is_dir"`
	Size     int64       `json:"size,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// GetCurrent returns the workspace path in Windows format.
// @Summary      获取工作目录
// @Description  获取当前 Agent 工作目录路径。
// @Tags         工作目录
// @Produce      json
// @Success      200 {object} model.APIEnvelope
// @Router       /workspace [get]
func (h *WorkspaceHandler) GetCurrent(c *gin.Context) {
	c.JSON(http.StatusOK, model.OK(gin.H{
		"path": toWin(h.currentDir),
	}))
}

// SetCurrent changes the workspace directory.
// @Summary      设置工作目录
// @Description  设置 Agent 的当前工作目录。路径不存在时会自动创建。
// @Tags         工作目录
// @Accept       json
// @Produce      json
// @Param        request body object{path=string} true "新的工作目录路径"
// @Success      200 {object} model.APIEnvelope
// @Failure      400 {object} model.APIEnvelope
// @Router       /workspace [post]
func (h *WorkspaceHandler) SetCurrent(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, "path is required"))
		return
	}

	raw := req.Path
	// Normalize path separators and clean
	raw = filepath.Clean(raw)
	// Ensure directory exists.
	checkPath := raw
	if len(raw) >= 2 && raw[1] == ':' {
		checkPath = "/" + strings.ToUpper(string(raw[0])) + "/" + strings.TrimLeft(raw[3:], `/\`)
	}
	os.MkdirAll(checkPath, 0755)
	h.currentDir = raw
	tool.SetWorkspaceRoot(raw)
	h.logger.Info("workspace changed", zap.String("path", raw), zap.String("container", checkPath))
	c.JSON(http.StatusOK, model.OK(gin.H{"path": raw}))
}

// ListTree 从当前工作目录开始返回目录树。
// @Summary      返回目录树
// @Description  以树形结构返回指定目录的内容（递归，默认深度 3 层）。
// @Tags         工作目录
// @Produce      json
// @Param        path query string false "起始目录路径（默认当前工作目录）"
// @Success      200 {object} model.APIEnvelope{data=handler.FileNode}
// @Failure      500 {object} model.APIEnvelope
// @Router       /workspace/tree [get]
func (h *WorkspaceHandler) ListTree(c *gin.Context) {
	depth := 3
	root := toContainer(h.currentDir)
	if p := c.Query("path"); p != "" {
		root = toContainer(p)
	}

	tree, err := h.buildTree(root, depth)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(tree))
}

// ListDrives 列出盘符（Windows）或根目录。
// @Summary      列出磁盘驱动器
// @Description  列出 Windows 系统上的所有可用磁盘驱动器。
// @Tags         工作目录
// @Produce      json
// @Success      200 {object} model.APIEnvelope
// @Router       /workspace/drives [get]
func (h *WorkspaceHandler) ListDrives(c *gin.Context) {
	var items []gin.H
	for _, drive := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(drive) + ":\\"
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			items = append(items, gin.H{"name": string(drive) + ":", "path": path + "\\", "is_dir": true})
		}
	}
	c.JSON(http.StatusOK, model.OK(items))
}

// ListDir lists immediate children of a directory.
// @Summary      列出目录内容
// @Description  列出指定目录下的文件和子目录（仅一层，不递归）。
// @Tags         工作目录
// @Produce      json
// @Param        path query string false "目录路径（默认当前工作目录）"
// @Success      200 {object} model.APIEnvelope
// @Failure      400 {object} model.APIEnvelope
// @Router       /workspace/dir [get]
func (h *WorkspaceHandler) ListDir(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		path = toContainer(h.currentDir)
	} else {
		path = toContainer(path)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, "无法读取目录: "+err.Error()))
		return
	}

	var nodes []*FileNode
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil && !info.IsDir() {
			size = info.Size()
		}
		nodes = append(nodes, &FileNode{
			Name:  e.Name(),
			Path:  filepath.Join(path, e.Name()),
			IsDir: e.IsDir(),
			Size:  size,
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].IsDir != nodes[j].IsDir {
			return nodes[i].IsDir
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})

	c.JSON(http.StatusOK, model.OK(nodes))
}

func (h *WorkspaceHandler) buildTree(root string, depth int) (*FileNode, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	node := &FileNode{
		Name:  filepath.Base(root),
		Path:  root,
		IsDir: info.IsDir(),
	}
	if !info.IsDir() {
		node.Size = info.Size()
		return node, nil
	}

	if depth <= 0 {
		return node, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return node, nil
	}

	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || strings.HasPrefix(e.Name(), "$") {
			continue
		}
		childPath := filepath.Join(root, e.Name())
		if e.IsDir() {
			child, err := h.buildTree(childPath, depth-1)
			if err == nil {
				node.Children = append(node.Children, child)
			}
		} else {
			info, _ := e.Info()
			size := int64(0)
			if info != nil {
				size = info.Size()
			}
			node.Children = append(node.Children, &FileNode{
				Name:  e.Name(),
				Path:  childPath,
				IsDir: false,
				Size:  size,
			})
		}
	}

	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})

	return node, nil
}
