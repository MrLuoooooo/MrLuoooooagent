# MCP Connector 规划书

## 1. 目标

让 GoAgentPro Agent 通过 MCP 协议动态接入外部 MCP Server 的工具。接入后 Agent ReAct 循环可透明调用，与本地工具无差异。

## 2. 核心组件

```
eino-ext/components/tool/mcp.GetTools() → 返回 []tool.BaseTool
mark3labs/mcp-go/client                → MCP 客户端 (SSE / stdio)
```

SSE 模式连远程服务，stdio 模式连本地子进程。

## 3. 架构位置

不改 Graph、不改 Tool 接口、不改现有工具。新增一个 MCPConnector 层：

```
config 层:   mcp_servers 列表
fx 层:       ProvideMCPConnector → 创建 client → GetTools → Tool.Register
Tool 层:     现有 Registry 不变，MCP 工具和其他工具平权注册
```

## 4. 配置设计

```yaml
# configs/config.yaml 新增
mcp:
  enabled: true
  servers:
    - name: "a-share"
      transport: "stdio"
      command: "python"
      args: ["-m", "a_share_mcp"]
      env: {}
    - name: "news-api"
      transport: "sse"
      url: "http://localhost:9999/sse"
    - name: "akshare"
      transport: "sse"
      url: "http://localhost:8888/sse"
```

Go 结构体（`internal/config/types.go`）：

```go
// **注意**：以下类型新增后，需在 Config struct 里加一行：
//   MCP  MCPConfig  `mapstructure:"mcp"`
// 否则 viper 读不到 mcp 段。

type MCPConfig struct {
    Enabled bool          `mapstructure:"enabled"`
    Servers []MCPServer   `mapstructure:"servers"`
}
type MCPServer struct {
    Name      string            `mapstructure:"name"`
    Transport string            `mapstructure:"transport"`   // stdio | sse
    Command   string            `mapstructure:"command"`     // stdio
    Args      []string          `mapstructure:"args"`        // stdio
    Env       map[string]string `mapstructure:"env"`         // stdio
    URL       string            `mapstructure:"url"`         // sse
}
```

Config struct 完整定义中追加：
```go
type Config struct {
    // ... 已有字段 ...
    MCP  MCPConfig  `mapstructure:"mcp"`
}
```

## 5. 类型适配层（关键）

eino-ext 的 `mcpTool.GetTools()` 返回 `[]eino_tool.BaseTool`（Eino 框架 Tool 接口），而项目里 `ToolRegistry` 接受 `internal/component/tool.Tool`（自定义接口）。两者方法签名完全一致但 Go 不隐式兼容。

**方案**：写一个轻量适配器，不换 Registry 接口，不动所有 30 个本地工具。

```go
// internal/component/mcp/connector.go
// baseToolAdapter 把 eino_tool.BaseTool 适配为 tool.Tool。
type baseToolAdapter struct {
    bt eino_tool.BaseTool
}
func (a *baseToolAdapter) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return a.bt.Info(ctx)
}
func (a *baseToolAdapter) InvokableRun(ctx context.Context, args string, opts ...eino_tool.Option) (string, error) {
    return a.bt.InvokableRun(ctx, args, opts...)
}
```

MCPConnector.Connect() 返回类型从 `[]tool.Tool` 改为 `[]eino_tool.BaseTool`，由 fx.Invoke 里做适配后注册。或者 `Connect()` 内部用适配器包好再返回 `[]tool.Tool`——后者更干净，调用方无感知。

## 6. 实现文件

新文件：

```
internal/component/mcp/
  connector.go          # MCPConnector: 管理 client 生命周期 + 工具拉取
  connector_test.go     # 测试 (mock MCP server)
```

改动文件：

```
internal/config/types.go                  # 新增 MCPConfig, MCPServer
configs/config.yaml                       # 新增 mcp 配置块
configs/config.docker.yaml                # 同步
internal/server/fx.go                     # ProvideMCPConnector + 生命周期钩子
go.mod                                    # 新增依赖
```

## 6. MCPConnector 设计

```go
// internal/component/mcp/connector.go
package mcp

type MCPConnector struct {
    servers []config.MCPServer
    clients []MCPClient // 持有的连接，用于关闭
    logger  *zap.Logger
}

func NewMCPConnector(cfg *config.Config, logger *zap.Logger) *MCPConnector

// Connect 逐个连接 MCP server，拉取工具列表。
// 返回已适配为 tool.Tool 的工具列表（内部用 baseToolAdapter 包装 BaseTool）。
func (c *MCPConnector) Connect(ctx context.Context) ([]tool.Tool, error)

// Close 关闭所有连接
func (c *MCPConnector) Close()
```

Connect 逻辑：
1. 遍历 config.MCP.Servers
2. 根据 transport 创建 client (SSE/stdio)
3. client.Start(ctx) + client.Initialize(ctx, initReq)
4. mcpTool.GetTools(ctx, &Config{Cli: client}) → []eino_tool.BaseTool
5. 每个 BaseTool 用 baseToolAdapter 适配 → tool.Tool
6. 收集所有工具返回

## 7. Fx 集成

```go
// fx.go 新增 ProvideMCPConnector
func ProvideMCPConnector(cfg *config.Config, logger *zap.Logger) *mcp_connector.MCPConnector {
    return mcp_connector.NewMCPConnector(cfg, logger)
}

// fx.go Module 里新增 Invoke：
fx.Invoke(func(connector *mcp_connector.MCPConnector, logger *zap.Logger) {
    tools, err := connector.Connect(context.Background())
    if err != nil {
        logger.Warn("mcp: connect failed", zap.Error(err))
        return
    }
    for _, t := range tools {
        tool.Register(t)
    }
    logger.Info("mcp: tools loaded", zap.Int("count", len(tools)))
})

// lifecycle 钩子：
fx.Invoke(func(lc fx.Lifecycle, connector *mcp_connector.MCPConnector) {
    lc.Append(fx.Hook{OnStop: func(ctx context.Context) error {
        connector.Close()
        return nil
    }})
})
```

## 8. 新增依赖

```
go get github.com/cloudwego/eino-ext/components/tool/mcp
go get github.com/mark3labs/mcp-go
go mod tidy
```

## 9. 接入后的调用链

```
用户问: "查一下茅台的财报"
  → Agent Graph ReAct 循环
  → LLM 决策调用 get_financial_data 工具
  → ToolsNode 找到 MCP 工具（来自 a-share-mcp server）
  → InvokableRun → MCP client → 远程 a-share-mcp → Baostock API
  → 返回 JSON 结果 → LLM 收到 tool_result → 继续推理
```

与本地工具的差异对 Agent 完全透明。

## 10. 执行步骤

1. go get 两个依赖 + go mod tidy
2. internal/config/types.go 加 MCPConfig/MCPServer，Config struct 加 `MCP MCPConfig`
3. configs/config.yaml 加 mcp 配置块（初始 empty，保证 viper 不报错）
4. internal/component/mcp/connector.go（含 baseToolAdapter）
5. internal/component/mcp/connector_test.go
6. internal/server/fx.go 注册 Provider + Invoke + Lifecycle
7. go build + go test 验证
