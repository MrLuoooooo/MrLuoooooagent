# GoAgentPro 代码规范与质量规则

> 基于 2026-05-26 面试官视角代码审查，提炼为可执行的开发规则。
> 每条规则包含：触发条件、违规示例、正确做法。

---

## 一、安全规则（P0 — 违反即阻塞合并）

### R1: 禁止在源码中硬编码凭证

**触发**：任何 `.go`、`.yaml`、`.json`、`.env` 文件包含 API Key、Token、密码等敏感信息。

```yaml
# ❌ 违规
model_provider:
  cloud:
    api_key: "sk-abc123..."

# ✅ 正确
model_provider:
  cloud:
    api_key: ""  # 通过环境变量 GOAGENT_MODEL_PROVIDER_CLOUD_API_KEY 注入
```

**检查点**：
- `configs/*.yaml` 中所有 `api_key`、`password`、`token` 字段必须为空或引用环境变量
- `docker-compose.yml` 中环境变量使用 `${VAR_NAME}` 语法，不直接写值
- 如果必须包含示例值，使用 `"your-api-key-here"` 等明显占位符

### R2: 模块路径必须可解析

**触发**：`go.mod` 中 `module` 字段为占位符。

```go
// ❌ 违规
module github.com/yourusername/goagentpro

// ✅ 正确
module github.com/<实际用户名或组织>/goagentpro
```

### R3: 禁止硬编码本地路径

**触发**：代码中出现 `C:\Users\xxx`、`D:\xxx` 等包含用户名或特定盘符的绝对路径。

```go
// ❌ 违规
v.SetDefault("server.allowed_dirs", []string{`D:\goagentpro`, `C:\Users\35617\Desktop`})

// ✅ 正确
// 通过配置文件或环境变量注入
v.SetDefault("server.allowed_dirs", []string{})
// 在 config.yaml 中由用户配置
```

---

## 二、健壮性规则（P1 — 修复后才能合并）

### R4: 禁止可能返回 nil 的函数无保护调用

**触发**：函数返回指针且可能为 nil，调用方未做 nil 检查。

```go
// ❌ 违规
func resolveModelProvider(...) *ResolvedConfig {
    ...
    return nil  // 可能返回 nil
}
// 调用方
mm := NewModelManager(resolved, ...) // resolved 可能为 nil

// ✅ 正确
func ProvideModelManager(resolved *ResolvedConfig, ...) *modelmanager.ModelManager {
    if resolved == nil {
        return modelmanager.NewModelManager(&stubChatModel{}, ...)
    }
    ...
}
```

### R5: 禁止 `atomic.Value` / `interface{}` 裸类型断言

**触发**：对 `atomic.Value.Load()` 或 `interface{}` 做直接类型断言，无 ok 检查。

```go
// ❌ 违规 — 存了非 *BatchPipeline 会 panic
ch := bp.(*pipeline.BatchPipeline).Execute(ctx, tasks)

// ✅ 正确
if p, ok := bp.(*pipeline.BatchPipeline); ok {
    ch := p.Execute(ctx, tasks)
} else {
    return "", fmt.Errorf("batch pipeline not initialized")
}
```

### R6: 禁止全局可变单例

**触发**：包级 `var` 变量作为注册表/状态容器。

```go
// ❌ 违规
var globalRegistry = &Registry{tools: make([]Tool, 0)}

// ✅ 正确 — 通过 fx 依赖注入
func ProvideToolRegistry() *Registry { return &Registry{} }
// 在 Module 中作为 fx.Provide 注册
```

**原因**：
- 测试无法隔离
- 无法在同一进程运行多实例
- 隐式依赖，破坏可测试性

### R7: ID 生成不能依赖单字符递增

**触发**：用 `string(rune('0'+i))` 或类似方式生成 ID。

```go
// ❌ 违规 — i >= 10 时 '0'+10 = ':' 不是数字
ID: "prompt_tc_" + string(rune('0'+i)),

// ✅ 正确
ID: fmt.Sprintf("prompt_tc_%d", i),
```

### R8: 禁止 handler 和 service 双重写数据

**触发**：同一个操作在 service 层和 handler 层各调一次持久化。

```go
// ❌ 违规 — assistant 消息被写两次
func (s *ChatService) Chat(...) {
    s.SaveAssistantMessage(convID, msg.Content, nil) // 第一次
}
func (h *ChatHandler) handleInvoke(...) {
    msg := h.svc.Chat(...)
    h.svc.SaveAssistantMessage(convID, msg.Content, nil) // 第二次！
}

// ✅ 正确 — 持久化只在 service 层做一次
```

### R9: 代码缩进必须一致

**触发**：同一函数内出现混乱的缩进层级。

```go
// ❌ 违规 — 第3行的语句实际在 if 块之外，但缩进误导
if win != "" {
    summary := tool.ReadWorkspaceSummary()
prompt += "\n## 当前工作目录: " + win  // 这行不在 if 内！

// ✅ 正确
if win != "" {
    summary := tool.ReadWorkspaceSummary()
    prompt += "\n## 当前工作目录: " + win + "\n目录内容：" + summary
}
```

### R10: SSE 流启动后不能再改 HTTP 状态码

**触发**：先写 SSE header（HTTP 200），之后再遇到错误无法返回正确的状态码。

```go
// ❌ 违规
h.setupSSE(c)  // 已发送 200 + SSE headers
stream, err := h.svc.ChatStream(...)
if err != nil {
    h.writeSSEEvent(w, model.StreamEvent{Type: model.EventError, ...})
    // HTTP 状态码仍是 200，但内容报错。语义混乱。
}

// ✅ 正确
stream, err := h.svc.ChatStream(...)
if err != nil {
    c.JSON(500, ...)  // 还未发 SSE header，可以正常返回错误
    return
}
h.setupSSE(c)  // 确认无错误后才发送 SSE header
```

---

## 三、工程质量规则（P2 — 下个迭代修复）

### R11: 错误必须结构化

**触发**：使用 `fmt.Errorf` / `errors.New` 作为唯一错误创建方式。

```go
// ❌ 违规 — 调用方无法区分错误类型
return nil, fmt.Errorf("rag chain invoke failed")

// ✅ 正确 — 定义 sentinel error 或自定义 error type
var ErrRAGUnavailable = errors.New("rag chain unavailable")

type RAGError struct {
    Op  string
    Err error
}
func (e *RAGError) Error() string { return fmt.Sprintf("rag %s: %v", e.Op, e.Err) }
func (e *RAGError) Unwrap() error { return e.Err }
```

### R12: `Config.Load()` 不能吞错误

**触发**：配置文件读取失败只打 warning 继续运行。

```go
// ❌ 违规
if err := v.ReadInConfig(); err != nil {
    log.Printf("Warning: failed to read config file: %v", err)
    // 无返回值，继续运行，用户不知道配置未加载
}

// ✅ 正确 — 区分情况
if err := v.ReadInConfig(); err != nil {
    if _, ok := err.(viper.ConfigFileNotFoundError); ok {
        log.Println("No config file found, using defaults and env vars")
    } else {
        return nil, fmt.Errorf("read config: %w", err)
    }
}
```

### R13: 审批/状态数据需要持久化

**触发**：业务关键数据仅存内存（`sync.Map`、`[]slice` 等）。

```go
// ❌ 违规
type ApprovalStore struct {
    mu    sync.RWMutex
    items []*model.ApprovalItem  // 重启全丢
}

// ✅ 正确 — 至少写入 ES 或文件
type ApprovalStore struct {
    es  *elasticsearch.Client
    idx string
}
```

### R14: 内存中的审批数据需要上限

**触发**：`ApprovalStore.items` 只增不减，无上限控制。

```go
// ❌ 违规 — 长期运行后可能 OOM
func (s *ApprovalStore) Add(item *model.ApprovalItem) {
    s.items = append(s.items, item)  // 永不清除
}

// ✅ 正确 — 定期清理已完成项，设置 maxItems
func (s *ApprovalStore) Add(item *model.ApprovalItem) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // 保留最近 N 条
    if len(s.items) >= s.maxItems {
        s.items = s.items[len(s.items)-s.maxItems+1:]
    }
    s.items = append(s.items, item)
}
```

### R15: ES 客户端必须配置超时和重试

**触发**：`elasticsearch.NewClient` 只传 `Addresses`。

```go
// ❌ 违规
return elasticsearch.NewClient(elasticsearch.Config{
    Addresses: cfg.VectorStore.Elasticsearch.Addresses,
})

// ✅ 正确
return elasticsearch.NewClient(elasticsearch.Config{
    Addresses: cfg.VectorStore.Elasticsearch.Addresses,
    Username:  cfg.VectorStore.Elasticsearch.Username,
    Password:  cfg.VectorStore.Elasticsearch.Password,
    RetryOnStatus: []int{502, 503, 504},
    MaxRetries:     3,
    Transport: &http.Transport{
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
})
```

### R16: 文件内容搜索不能全量读内存 + 暴力扫描

**触发**：`os.ReadFile` 整个文件 + `strings.Contains` + 无大小限制（业务默认最多 5MB 只是 `searchByContent` 自己的约束，存在隐患）。

```go
// ❌ 违规
data, _ := os.ReadFile(p)
lines := strings.Split(string(data), "\n")
for _, line := range lines {
    if strings.Contains(line, content) { ... }
}

// ✅ 正确 — 流式读取 + 限制大小
f, _ := os.Open(p)
defer f.Close()
scanner := bufio.NewScanner(f)
scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 单行 1MB
for scanner.Scan() {
    if bytes.Contains(scanner.Bytes(), []byte(content)) { ... }
}
```

### R17: 排序用标准库

**触发**：手写冒泡排序。

```go
// ❌ 违规 — O(n²)
func sortLines(lines []string) {
    for i := 0; i < len(lines); i++ {
        for j := i + 1; j < len(lines); j++ {
            if lines[i] > lines[j] { ... }
        }
    }
}

// ✅ 正确 — O(n log n)
sort.Strings(lines)
```

### R18: 版本号必须一致

**触发**：代码注释声明的版本与实际返回不一致。

```go
// ❌ 违规
// @version 4.1.2  (Swagger 注释)
c.JSON(200, gin.H{"version": "1.0.0"})  // health 端点

// ✅ 正确 — 使用常量
const AppVersion = "4.1.2"
// @version 4.1.2
c.JSON(200, gin.H{"version": AppVersion})
```

---

## 四、代码风格规则

### R19: 禁止忽略错误返回值

```go
// ❌ 违规
os.WriteFile(filepath.Join(configDir, "config.yaml"), yamlContent, 0644)
os.MkdirAll(configDir, 0755)

// ✅ 正确
if err := os.MkdirAll(configDir, 0755); err != nil {
    t.Fatalf("create config dir: %v", err)
}
```

### R20: 禁止 `.exe`、`.stackdump` 进入工作目录根

**触发**：编译产物、崩溃文件出现在项目根目录。

**解决**：添加 `.gitignore`：
```
/bin/
*.exe
*.stackdump
*.test.exe
```

### R21: 函数内局部变量的作用域要清晰

```go
// ❌ 违规 — ctx 被遮蔽
ctx, cancel := context.WithTimeout(ctx, ...)
defer cancel()
// ...
ctx, cancel = context.WithTimeout(ctx, ...) // 遮蔽外层 ctx

// ✅ 正确 — 用不同变量名
execCtx, execCancel := context.WithTimeout(ctx, ...)
defer execCancel()
```

---

## 附录：检查清单

合并前必须确认：

- [ ] R1: config.yaml 无明文 API Key
- [ ] R2: go.mod module 路径正确
- [ ] R3: 无硬编码个人路径
- [ ] R4: nil 返回函数调用方有检查
- [ ] R5: 所有类型断言有 ok 检查
- [ ] R6: 未新增全局可变单例
- [ ] R8: handler 未重复持久化
- [ ] R9: 代码缩进正确
- [ ] R10: SSE header 在确认无错误后发送
- [ ] R19: 无忽略的 error 返回值
