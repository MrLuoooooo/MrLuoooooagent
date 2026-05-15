# GoAgentPro — 安全加固 + 代码质量修复任务书

> **生成时间**: 2026-05-13 15:05 CST  
> **用途**: 将此文件完整交付给开发 Agent，按优先级修复项目中的安全问题、架构缺陷和测试缺口

---

## 一、项目当前状态

| 维度 | 状态 | 详情 |
|------|------|------|
| Agent Graph | ✅ 完整 | ReAct 循环 + 10 个工具 + System Prompt |
| 文件系统工具 | ✅ 已实现 | fs.go 8 个工具（读/写/编辑/删除/列表/搜索/创建/信息） |
| Bash 工具 | ✅ 已实现 | execute_command + write_and_execute |
| OpenAI Model | ✅ 已修复 | tools 传输、stream tool_calls delta、options 处理 |
| 编译 | ✅ 通过 | `go build ./...` 零错误 |
| 测试 | ✅ 通过 | 15 个测试文件全部通过 |
| 前端 Agent 模式 | ✅ 已实现 | Agent 切换按钮 + SSE tool_call/tool_result 事件处理 |
| 持久化 | ✅ ES | 对话 + 消息 + 文档 + 向量 4 个索引 |

---

## 二、修复清单（按优先级执行）

---

### 🚨 P0 — 立即修复

#### 任务 1: 移除 config.yaml 明文 API Key

**涉及文件**:
- `D:\goagentpro\configs\config.yaml`
- `D:\goagentpro\.gitignore`

**操作**:
1. 从 `config.yaml` 中移除以下行：
   ```yaml
   model_provider:
     cloud:
       api_key: "sk-9085ad86a2a046a7b43a18117115e572"
   ```
   替换为：
   ```yaml
   model_provider:
     cloud:
       api_key: ""  # 通过环境变量 GOAGENT_MODEL_PROVIDER_CLOUD_API_KEY 设置
   ```

2. 检查 `.gitignore`，确保包含：
   ```
   configs/config.yaml
   .env
   *.local.yaml
   ```

3. 创建一个 `configs/config.example.yaml`（作为模板，不含真实 key）

**验收标准**:
- ❌ `config.yaml` 中不包含任何 `sk-` 开头的字符串
- ✅ `configs/config.example.yaml` 存在且可作模板
- ✅ `go build ./...` 通过

---

#### 任务 2: 加固 Bash 工具安全过滤

**涉及文件**:
- `D:\goagentpro\internal\component\tool\bash.go`

**当前问题**:
- 字符串前缀匹配（`strings.HasPrefix`）可被引号/空格/拆分绕过
- 缺少对 `exec.Command.Args` 级别的检查

**修改方案**：

重构 `blockedPrefixes` 为基于 tokenize 后的命令检查：

```go
// 替换原有的 blockedPrefixes + InvokableRun 中的安全检查部分
// 改为：

// blockedCommands 是被禁止的"命令"列表（小写，不包含参数）
var blockedCommands = map[string]string{
    "rm":     "禁止使用 rm 删除文件系统",
    "format": "禁止磁盘格式化操作",
    "fdisk":  "禁止磁盘分区操作",
    "mkfs":   "禁止创建文件系统",
    "dd":     "禁止低级磁盘写入",
    "shutdown": "禁止系统关机",
    "reboot":   "禁止系统重启",
    "poweroff": "禁止系统关机",
    "halt":     "禁止系统停止",
    "sudo":     "禁止权限提升",
    "chmod":    "禁止修改文件权限",
    "chown":    "禁止修改文件所有者",
    "del":      "禁止删除系统文件",
    "rd":       "禁止删除目录",
}

// 使用 exec.Command 的 Args 拆解来做安全检查
func isSafeCommand(command string) (bool, string) {
    // 用 cmd /c 的 tokenize 方式解析
    // 通过 Go 的 exec.Command 本身的 tokenizer 规则
    // 但这里简单起见: 用 strings.Fields 拆出 token
    tokens := strings.Fields(strings.TrimSpace(command))
    if len(tokens) == 0 {
        return false, "空命令"
    }
    
    // 跳过 cmd /c 前缀
    startIdx := 0
    lowerTokens := make([]string, len(tokens))
    for i, t := range tokens {
        lowerTokens[i] = strings.ToLower(strings.Trim(t, `"'`))
    }
    
    firstCmd := lowerTokens[startIdx]
    // 如果是 cmd /c，取第二个
    if firstCmd == "cmd" && len(lowerTokens) > startIdx+1 {
        startIdx++
        firstCmd = lowerTokens[startIdx]
    }
    // 如果是 cmd /c 中的 /c
    if firstCmd == "/c" && len(lowerTokens) > startIdx+1 {
        startIdx++
        firstCmd = lowerTokens[startIdx]
    }
    
    reason, blocked := blockedCommands[firstCmd]
    if blocked {
        return false, reason
    }
    
    // 额外检查：任何包含 rm 且包含 -rf 的
    full := strings.Join(lowerTokens[startIdx:], " ")
    if strings.Contains(full, "rm ") && (strings.Contains(full, "-rf") || strings.Contains(full, "-r -f")) {
        return false, "禁止递归强制删除"
    }
    // 检查写入磁盘设备
    if strings.Contains(full, "> /dev/sd") || strings.Contains(full, ">\\\\.\\") {
        return false, "禁止直接写入磁盘设备"
    }
    
    return true, ""
}
```

**验收标准**:
- ✅ `rm -rf /c/*` → 拦截
- ✅ `rm -rf "/c/Users"` → 拦截（引号绕过）
- ✅ `cmd /c rm -r -f /c/windows` → 拦截（cmd 前缀）
- ✅ `npm run build` → 放行
- ✅ `go test ./...` → 放行
- ✅ `dir /s /b` → 放行
- ✅ `go build ./...` 通过
- ✅ `go test ./internal/component/tool/...` 通过

---

### ⚠️ P1 — 尽快修复

#### 任务 3: 注册 Callback 日志到 fx 启动流程

**涉及文件**:
- `D:\goagentpro\internal\server\fx.go`
- `D:\goagentpro\internal\server\router.go`（或新建 init）

**操作**：

在 `fx.go` 中新增一个 `fx.Invoke` 来注册全局 callback：

```go
// 在 Module 的 fx.Invoke 列表中新增
fx.Invoke(func(logger *zap.Logger) {
    handler := callback.NewLoggingCallback(logger)
    callbacks.AppendGlobalHandlers(handler)
    logger.Info("global callback handler registered")
}),
```

**验收标准**:
- ✅ 应用启动时日志输出 `global callback handler registered`
- ✅ ChatModel/Retriever/Indexer/Tool 执行时输出 `component start/end` 日志
- ✅ `go build ./...` 通过

---

#### 任务 4: 为 openaimodel + openaiembed + config 添加单元测试

**涉及文件**:
- `D:\goagentpro\internal\component\openaimodel\openai_chat_model_test.go`（新建）
- `D:\goagentpro\internal\component\openaiembed\openai_embedder_test.go`（新建）
- `D:\goagentpro\internal\config\config_test.go`（新建）

**openaimodel 测试要求**（至少 4 个用例）：
```go
// 1. TestBindTools_StoresTools — BindTools 正确存储 tools
// 2. TestBindTools_WithParams — BindTools 正确转换 JSON Schema
// 3. TestGenerate_RequestFormat — 验证请求体正确包含 Model/Messages/Stream=false/Tools
// 4. TestConvertMessages — convertMessages 正确处理 ToolCalls 和 ToolCallID
// 5. TestConvertMessages_WithToolCalls — 工具调用消息正确转换为 OpenAI 格式
```

使用 mock HTTP 服务器（`httptest.NewServer`）模拟 OpenAI API 响应。

**openaiembed 测试要求**（至少 2 个用例）：
```go
// 1. TestEmbedStrings_ReturnsVectors — 正常路径返回正确的向量维度
// 2. TestEmbedStrings_APICall — 验证请求体结构和 Authorization header
```

**config 测试要求**（至少 3 个用例）：
```go
// 1. TestSetDefaults — 验证默认值正确设置
// 2. TestEnvOverride — 验证环境变量覆盖 YAML 配置
// 3. TestLoadFromFile — 验证 YAML 文件正确加载（需要创建测试用临时文件）
```

**验收标准**:
- ✅ 至少 9 个新测试用例
- ✅ `go test ./internal/component/openaimodel/...` 通过
- ✅ `go test ./internal/component/openaiembed/...` 通过
- ✅ `go test ./internal/config/...` 通过
- ✅ 测试不依赖外部服务（使用 mock/fake）

---

#### 任务 5: 实现 Server graceful shutdown

**涉及文件**:
- `D:\goagentpro\cmd\server\main.go`

**修改方案**：

```go
func startServer(lc fx.Lifecycle, cfg *config.Config, logger *zap.Logger, 
    engine *gin.Engine, esClient *elasticsearch.Client) {
    gin.SetMode(cfg.Server.Mode)
    
    srv := &http.Server{
        Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
        Handler: engine,
    }
    
    lc.Append(fx.Hook{
        OnStart: func(ctx context.Context) error {
            logger.Info("Starting server", zap.String("addr", srv.Addr))
            go func() {
                if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
                    logger.Error("Server failed", zap.Error(err))
                }
            }()
            return nil
        },
        OnStop: func(ctx context.Context) error {
            logger.Info("Shutting down server gracefully...")
            // 1. 关闭 HTTP 服务器（等待最多 10 秒）
            shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
            defer cancel()
            if err := srv.Shutdown(shutdownCtx); err != nil {
                logger.Error("HTTP server shutdown error", zap.Error(err))
            }
            // 2. 关闭 ES 连接
            // 3. Flush 日志缓冲区
            logger.Info("Server stopped")
            return nil
        },
    })
}
```

**注意**: `fx.go` 需要 Provide `*elasticsearch.Client` 到 `startServer` 参数中。

**验收标准**:
- ✅ `go build ./...` 通过
- ✅ Ctrl+C 发送 SIGINT 时，服务器等待正在进行的请求完成后再退出
- ✅ 日志输出 `Shutting down server gracefully...`

---

### 🟡 P2 — 改进

#### 任务 6: 统一 Tool 注册到 fx 生命周期

**涉及文件**:
- `D:\goagentpro\internal\server\fx.go`

**当前问题**：Tool 注册混在 `ProvideAgentGraph` 中，职责混合。

**修改方案**：
1. 新建 `ProvideToolRegistry` 函数，在 fx 启动阶段注册所有工具
2. `ProvideAgentGraph` 只做"从已注册的工具构建 ToolsNode + 绑定 + 创建图"

```go
// 新增
func ProvideToolRegistry(cfg *config.Config, ragChain compose.Runnable[string, *eino_schema.Message]) {
    tool.Register(&tool.ReadFileTool{})
    tool.Register(&tool.WriteFileTool{})
    tool.Register(&tool.EditFileTool{})
    tool.Register(&tool.DeleteFileTool{})
    tool.Register(&tool.ListDirectoryTool{})
    tool.Register(&tool.CreateDirectoryTool{})
    tool.Register(&tool.SearchFilesTool{})
    tool.Register(&tool.GetFileInfoTool{})
    tool.Register(tool.NewBashTool(cfg.Server.AllowedDirs))
    tool.Register(tool.NewWriteAndExecuteTool(cfg.Server.AllowedDirs))
    tool.Register(&tool.DateTimeTool{})
    tool.Register(tool.NewWebSearchTool(...))
    tool.Register(tool.NewRAGTool(func(ctx context.Context, query string) (string, error) { ... }))
}
```

**验收标准**:
- ✅ Tool 注册不在 Provide 函数体里，而是独立的 fx.Invoke
- ✅ `go build ./...` 通过

---

#### 任务 7: ConversationService 抽象为接口

**涉及文件**:
- `D:\goagentpro\internal\service\conversation.go`
- `D:\goagentpro\internal\handler\conversation.go`
- `D:\goagentpro\internal\server\fx.go`

**修改方案**：
```go
// ConversationStore 接口
type ConversationStore interface {
    Create(ctx context.Context, id string, title string) error
    List(ctx context.Context) ([]store.ConversationMeta, error)
    Load(ctx context.Context, conversationID string) ([]*schema.Message, error)
    Save(ctx context.Context, conversationID string, msgs []*schema.Message) error
    Delete(ctx context.Context, conversationID string) error
}

// ConversationService 不再直接依赖 *store.ESConversationStore
type ConversationService struct {
    store  ConversationStore
    logger *zap.Logger
}
```

**验收标准**:
- ✅ `ConversationService` 不直接引用 `*store.ESConversationStore`
- ✅ `go build ./...` 通过
- ✅ `go test ./...` 全部通过

---

#### 任务 8: 前端限制 CORS + 添加 rate limiter

**涉及文件**:
- `D:\goagentpro\internal\server\middleware\cors.go`
- 新增 `D:\goagentpro\internal\server\middleware\ratelimit.go`

**CORS 修改**：可配置 origin，不硬编码 `*`。

**Rate limiter**：基于 IP 的简单令牌桶，每秒 10 请求。

---

## 三、执行顺序

```
第一轮（P0 — 安全）:
  任务 1: 移除 config.yaml 明文 key        → 5 分钟
  任务 2: 加固 Bash 安全过滤               → 20 分钟
  └── go build + go test 验证

第二轮（P1 — 质量）:
  任务 3: 注册 Callback 日志               → 10 分钟
  任务 4: 补写测试（openaimodel/openaiembed/config）→ 30 分钟
  任务 5: Server graceful shutdown         → 15 分钟
  └── go build + go test 验证

第三轮（P2 — 架构）:
  任务 6: Tool 注册独立到 fx 生命周期      → 15 分钟
  任务 7: ConversationService 接口抽象      → 20 分钟
  任务 8: CORS + rate limiter              → 15 分钟
  └── go build + go test 验证
```

---

## 四、输出要求

1. 每完成一个任务用 `✅/❌/🟡` 表格汇报
2. 代码改动输出完整的 diff 上下文
3. 严格遵守「检查 → 修改 → 测试 → 审查」四步法
4. 每次 `go build ./...` + `go vet ./...` + `go test ./...` 验证通过后才能进行下一步
