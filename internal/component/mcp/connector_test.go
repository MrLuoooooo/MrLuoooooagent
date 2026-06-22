package mcp

import (
	"context"
	"testing"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/tool"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"go.uber.org/zap"
)

func TestNewConnector(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{Enabled: false, Servers: nil},
	}
	conn := NewConnector(cfg, zap.NewNop())
	if conn == nil {
		t.Fatal("expected non-nil connector")
	}
}

func TestConnector_Disabled(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{Enabled: false, Servers: nil},
	}
	conn := NewConnector(cfg, zap.NewNop())
	tools, err := conn.Connect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestConnector_Close_Idempotent(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{Enabled: false, Servers: nil},
	}
	conn := NewConnector(cfg, zap.NewNop())
	conn.Close()
	conn.Close() // 不 panic
}

func TestBaseToolAdapter(t *testing.T) {
	// adapter 满足 tool.Tool 接口
	var _ tool.Tool = (*baseToolAdapter)(nil)
}
