package mcp

import (
	"context"
	"fmt"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/tool"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	eino_tool "github.com/cloudwego/eino/components/tool"
	mcpTool "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/schema"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"go.uber.org/zap"
)

// Connector 管理 MCP server 连接生命周期。
type Connector struct {
	servers []config.MCPServer
	clients []closer
	logger  *zap.Logger
}

type closer interface{ Close() error }

// NewConnector —
func NewConnector(cfg *config.Config, logger *zap.Logger) *Connector {
	return &Connector{servers: cfg.MCP.Servers, logger: logger}
}

// Connect 逐个连接 MCP server，拉取工具列表。
// 任一 server 失败不阻塞后续，仅日志记录。
func (c *Connector) Connect(ctx context.Context) ([]tool.Tool, error) {
	var all []tool.Tool
	for _, srv := range c.servers {
		tools, err := c.connectOne(ctx, srv)
		if err != nil {
			c.logger.Warn("mcp: connect server failed",
				zap.String("server", srv.Name), zap.Error(err))
			continue
		}
		all = append(all, tools...)
		c.logger.Info("mcp: tools loaded",
			zap.String("server", srv.Name), zap.Int("count", len(tools)))
	}
	return all, nil
}

func (c *Connector) connectOne(ctx context.Context, srv config.MCPServer) ([]tool.Tool, error) {
	mcpClient, err := c.dial(ctx, srv)
	if err != nil {
		return nil, err
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "goagent-pro",
		Version: "4.3.0",
	}
	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("mcp init: %w", err)
	}

	rawTools, err := mcpTool.GetTools(ctx, &mcpTool.Config{Cli: mcpClient})
	if err != nil {
		return nil, fmt.Errorf("mcp get tools: %w", err)
	}

	tools := make([]tool.Tool, 0, len(rawTools))
	for _, bt := range rawTools {
		it, ok := bt.(eino_tool.InvokableTool)
		if !ok {
			c.logger.Warn("mcp: tool is non-invokable, skipped",
				zap.String("server", srv.Name))
			continue
		}
		tools = append(tools, &baseToolAdapter{it: it})
	}
	return tools, nil
}

func (c *Connector) dial(ctx context.Context, srv config.MCPServer) (*client.Client, error) {
	switch srv.Transport {
	case "sse":
		if srv.URL == "" {
			return nil, fmt.Errorf("mcp sse: url required")
		}
		cc, err := client.NewSSEMCPClient(srv.URL)
		if err != nil {
			return nil, fmt.Errorf("mcp sse: %w", err)
		}
		if err := cc.Start(ctx); err != nil {
			return nil, fmt.Errorf("mcp sse start: %w", err)
		}
		c.clients = append(c.clients, cc)
		return cc, nil

	case "stdio":
		if srv.Command == "" {
			return nil, fmt.Errorf("mcp stdio: command required")
		}
		env := make([]string, 0, len(srv.Env))
		for k, v := range srv.Env {
			env = append(env, k+"="+v)
		}
		cc, err := client.NewStdioMCPClient(srv.Command, env, srv.Args...)
		if err != nil {
			return nil, fmt.Errorf("mcp stdio: %w", err)
		}
		// NewStdioMCPClient 已自动 Start，无需显式调
		c.clients = append(c.clients, cc)
		return cc, nil

	default:
		return nil, fmt.Errorf("mcp: unknown transport %q", srv.Transport)
	}
}

// Close 关闭所有连接。
func (c *Connector) Close() {
	for _, cli := range c.clients {
		if err := cli.Close(); err != nil {
			c.logger.Warn("mcp: close failed", zap.Error(err))
		}
	}
	c.clients = nil
	c.logger.Info("mcp: all connections closed")
}

// ── baseToolAdapter ────────────────────────────

// baseToolAdapter 把 eino_tool.InvokableTool 适配为 tool.Tool。
type baseToolAdapter struct {
	it eino_tool.InvokableTool
}

func (a *baseToolAdapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return a.it.Info(ctx)
}

func (a *baseToolAdapter) InvokableRun(ctx context.Context, args string, opts ...eino_tool.Option) (string, error) {
	return a.it.InvokableRun(ctx, args, opts...)
}
