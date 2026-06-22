package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
)

// McpStore 管理 MCP 服务器配置的持久化存储（JSON 文件）。
type McpStore struct {
	mu      sync.Mutex
	path    string
	data    *mcpStoreData
}

type mcpStoreData struct {
	Enabled bool               `json:"enabled"`
	Servers []config.MCPServer `json:"servers"`
}

// NewMcpStore 创建并加载 MCP 存储。
func NewMcpStore(dataDir string) *McpStore {
	s := &McpStore{path: filepath.Join(dataDir, "mcp_servers.json")}
	s.load()
	return s
}

func (s *McpStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = &mcpStoreData{Enabled: false, Servers: []config.MCPServer{}}
	if b, err := os.ReadFile(s.path); err == nil {
		json.Unmarshal(b, s.data)
	}
}

func (s *McpStore) saveLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(s.path, b, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// Load 返回所有服务器配置。
func (s *McpStore) Load() ([]config.MCPServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]config.MCPServer, len(s.data.Servers))
	copy(cp, s.data.Servers)
	return cp, nil
}

// IsEnabled 返回 MCP 是否启用。
func (s *McpStore) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Enabled
}

// Upsert 创建或更新服务器配置（按 name 去重）。
func (s *McpStore) Upsert(srv config.MCPServer) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.data.Servers {
		if existing.Name == srv.Name {
			s.data.Servers[i] = srv
			return s.saveLocked()
		}
	}
	s.data.Servers = append(s.data.Servers, srv)
	return s.saveLocked()
}

// Remove 删除服务器配置。
func (s *McpStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, srv := range s.data.Servers {
		if srv.Name == name {
			s.data.Servers = append(s.data.Servers[:i], s.data.Servers[i+1:]...)
			return s.saveLocked()
		}
	}
	return fmt.Errorf("mcp server %q not found", name)
}

// SetEnabled 切换 MCP 启用状态。
func (s *McpStore) SetEnabled(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Enabled = enabled
	return s.saveLocked()
}
