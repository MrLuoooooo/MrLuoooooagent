package mcp

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

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
// 非并发安全——Connect() 和 Close() 不可在多个 goroutine 中同时调用。
type Connector struct {
	servers []config.MCPServer
	clients []closer
	logger  *zap.Logger
}

type closer interface{ Close() error }

// NewConnector —
func NewConnector(cfg *config.Config, logger *zap.Logger) *Connector {
	if logger == nil {
		logger = zap.NewNop()
	}
	servers := []config.MCPServer{}
	if cfg != nil {
		servers = cfg.MCP.Servers
	}
	return &Connector{servers: servers, logger: logger}
}

// ConnectResult 单个 server 的连接结果。
type ConnectResult struct {
	Server config.MCPServer
	Tools  []tool.Tool
	Error  string
}

// ConnectOne 连接单个 MCP server，返回工具和可能的错误。
func (c *Connector) ConnectOne(ctx context.Context, srv config.MCPServer) (*ConnectResult, error) {
	tools, err := c.connectOne(ctx, srv)
	if err != nil {
		return &ConnectResult{Server: srv, Error: err.Error()}, err
	}
	return &ConnectResult{Server: srv, Tools: tools}, nil
}

// Connect 逐个连接 MCP server，拉取工具列表。
// 任一 server 失败不阻塞后续。全部失败时返回聚合错误。
func (c *Connector) Connect(ctx context.Context) ([]tool.Tool, error) {
	var all []tool.Tool
	var errs []error
	for _, srv := range c.servers {
		tools, err := c.connectOne(ctx, srv)
		if err != nil {
			c.logger.Warn("mcp: connect server failed",
				zap.String("server", srv.Name), zap.Error(err))
			errs = append(errs, fmt.Errorf("%s: %w", srv.Name, err))
			continue
		}
		all = append(all, tools...)
		c.logger.Info("mcp: tools loaded",
			zap.String("server", srv.Name), zap.Int("count", len(tools)))
	}
	if len(errs) > 0 && len(all) == 0 {
		return nil, fmt.Errorf("mcp: all %d servers failed: %w", len(errs), errors.Join(errs...))
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
		info, err := it.Info(ctx)
		if err != nil {
			c.logger.Warn("mcp: tool info failed, skipped",
				zap.String("server", srv.Name), zap.Error(err))
			continue
		}
		if warns := validateToolSchema(info); len(warns) > 0 {
			for _, w := range warns {
				c.logger.Warn("mcp: tool schema warning",
					zap.String("server", srv.Name),
					zap.String("tool", info.Name),
					zap.String("issue", w))
			}
		}
		tools = append(tools, &baseToolAdapter{it: it})
	}
	return tools, nil
}

func (c *Connector) dial(ctx context.Context, srv config.MCPServer) (*client.Client, error) {
	switch srv.Transport {
	case "sse":
		return c.dialSSE(ctx, srv)
	case "stdio":
		return c.dialStdio(srv)
	case "agent":
		return nil, fmt.Errorf("agent-managed project (path: %s) — no auto-connect, Agent will read files on demand", srv.URL)
	default:
		return nil, fmt.Errorf("mcp: unknown transport %q", srv.Transport)
	}
}

func (c *Connector) dialSSE(ctx context.Context, srv config.MCPServer) (*client.Client, error) {
	if srv.URL == "" {
		return nil, fmt.Errorf("mcp sse: url required")
	}
	cc, err := client.NewSSEMCPClient(srv.URL)
	if err != nil {
		return nil, fmt.Errorf("mcp sse: %w", err)
	}
	if err := cc.Start(ctx); err != nil {
		cc.Close() // 释放已分配资源
		return nil, fmt.Errorf("mcp sse start: %w", err)
	}
	c.clients = append(c.clients, cc)
	return cc, nil
}

func (c *Connector) dialStdio(srv config.MCPServer) (*client.Client, error) {
	if srv.Command == "" {
		return nil, fmt.Errorf("mcp stdio: command required")
	}
	// 排序 key 保证环境变量顺序确定性
	keys := make([]string, 0, len(srv.Env))
	for k := range srv.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, k := range keys {
		env = append(env, k+"="+srv.Env[k])
	}

	cc, err := client.NewStdioMCPClient(srv.Command, env, srv.Args...)
	if err != nil {
		return nil, fmt.Errorf("mcp stdio: %w", err)
	}
	// NewStdioMCPClient 已自动 Start，无需显式调用
	c.clients = append(c.clients, cc)
	return cc, nil
}

// Close 关闭所有连接。幂等可重入。
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
	if a.it == nil {
		return nil, fmt.Errorf("mcp adapter: nil underlying tool")
	}
	return a.it.Info(ctx)
}

func (a *baseToolAdapter) InvokableRun(ctx context.Context, args string, opts ...eino_tool.Option) (string, error) {
	if a.it == nil {
		return "", fmt.Errorf("mcp adapter: nil underlying tool")
	}
	return a.it.InvokableRun(ctx, args, opts...)
}

// validateToolSchema 校验 MCP 工具的 description 质量。
// 返回警告列表（不阻止注册，仅告警）。
func validateToolSchema(info *schema.ToolInfo) []string {
	var warns []string

	if info.Desc == "" {
		warns = append(warns, "description is empty, LLM may not know when to call this tool")
	}

	if len([]rune(info.Desc)) > 0 && len([]rune(info.Desc)) < 10 {
		warns = append(warns, fmt.Sprintf("description too short (%d chars): %q", len([]rune(info.Desc)), info.Desc))
	}

	genericWords := []string{"can help", "useful", "helpful", "a tool that", "does things"}
	for _, w := range genericWords {
		if containsWord(info.Desc, w) && len([]rune(info.Desc)) < 50 {
			warns = append(warns, fmt.Sprintf("description is generic (%q), add specific function details", info.Desc))
			break
		}
	}
	return warns
}

func containsWord(text, word string) bool {
	return strings.Contains(text, word)
}
