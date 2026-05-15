# GoAgentPro — Agent 接手开发完整提示词

> **场景**: #15 日常开发  
> **生成时间**: 2026-05-13 08:34 CST  
> **用途**: 将此文件完整交付给 AI Agent，使其能无缝接手 GoAgentPro 项目的后续开发工作

---

## 一、项目身份卡

| 项目 | GoAgentPro |
|------|-----------|
| 本质 | RAG + Agent 智能对话系统，基于 CloudWeGo Eino 框架构建 |
| 后端 | Go (Gin + Eino v0.8.13 + Elasticsearch 8.x + uber-fx DI) |
| 前端 | React 18 + TypeScript 5.8 + Vite 6 + Tailwind CSS 4 |
| 部署 | Docker Compose (ES + Go App + Nginx 三容器) |
| 仓库 | `D:\goagentpro` |
| 模块名 | `github.com/yourusername/goagentpro` |

---

## 二、关键技术约束（必须遵守）

### 2.1 环境约束

| 约束 | 说明 |
|------|------|
| **OS** | Windows — Git Bash + PowerShell 混合使用 |
| **Docker 网络** | `localhost` 在 WSL/Docker 环境下可能无法解析到 `127.0.0.1` |
| **前端访问地址** | **必须用** `http://127.0.0.1:8081`，**绝不能**用 `localhost:8081` |
| **桌面路径** | `C:\Users\35617\Desktop` |
| **项目路径** | `D:\goagentpro` |
| **Node.js** | `D:\nodejs\node.exe` (v24.14.0) |
| **Go** | 1.23.0 |

### 2.2 架构约束

| 约束 | 说明 |
|------|------|
| **严禁 Ant Design / MUI** | 前端 UI 库只用 Tailwind CSS + lucide-react 图标 |
| **无全局状态管理库** | 禁止 Redux/Zustand/React Query — 用自定义 hooks |
| **无 HTTP 客户端库** | 用原生 `fetch`，不用 axios |
| **前端构建** | 手动 `cd web && npm install && npm run build`，Docker Compose **不会**自动构建前端 |
| **后端 DI** | 用 uber-fx，不要手动 new/wire |
| **LLM 编排** | 用 Eino framework (compose.Chain / compose.Graph)，不自己造轮子 |
| **配置管理** | 用 viper，环境变量前缀 `GOAGENT_`，YAML 配置 |

### 2.3 Eino 框架特殊性

| 知识点 | 说明 |
|--------|------|
| 当前版本 | `cloudwego/eino v0.8.13` |
| 未使用 ADK 模式 | 项目目前直接使用 Eino 的底层 API（compose.Graph, compose.Chain），未使用 Eino ADK 上层模式 |
| 执行引擎 | 默认使用 Pregel 模式（`compose/graph.go:645` 的 `runType = runTypePregel`），Pregel 支持有环图 |
| 循环上限 | `maxRunSteps = len(chanSubscribeTo) + 10 = 14`，防止死循环 |
| 核心执行循环 | `compose/graph_run.go:241` 的 `for step := 0; ; step++` |
| ToolCall 路由 | 通过 graph Branch 判断 `ToolCalls > 0` 自动路由到 ToolsNode |
| Callback | 用法：`callbacks.NewHandlerBuilder().OnStartFn().OnEndFn().OnErrorFn().Build()` |

---

## 三、代码库地图（完整文件清单）

### 3.1 后端 Go 源码 (`internal/` + `cmd/`)

```
D:\goagentpro\
├── cmd/server/main.go              # 入口: fx.New(server.Module).Run()
│
├── internal/
│   ├── config/
│   │   ├── types.go                # 所有配置结构体 (Config, ServerConfig, AuthConfig 等)
│   │   └── config.go               # Load() + setDefaults() + ENV 覆盖
│   │
│   ├── server/
│   │   ├── fx.go                   # DI 容器: Module 定义 (20 个 Provide + fx.Invoke)
│   │   │                            # 含 resolveModelProvider() 自动检测 Ollama
│   │   ├── router.go               # Gin 路由: /health + 8 个 API 端点
│   │   ├── middleware/
│   │   │   ├── auth.go             # Bearer Token 验证 (非强阻断)
│   │   │   ├── cors.go             # CORS (AllowAllOrigins)
│   │   │   └── logging.go          # Zap 请求日志
│   │
│   ├── handler/
│   │   ├── chat.go                 # ChatHandler: Chat() + handleInvoke/Stream/Agent
│   │   ├── conversation.go         # ConversationHandler: CRUD
│   │   └── document.go             # DocumentHandler: Upload/Delete/List (后两者 stub)
│   │
│   ├── service/
│   │   ├── chat.go                 # ChatService: 编排 RAG/Agent + 消息持久化
│   │   ├── conversation.go         # ConversationService: 包装 ESConversationStore
│   │   └── document.go             # DocumentService: 包装 DocIngestionChain
│   │
│   ├── model/
│   │   ├── chat.go                 # ChatRequest/Response/StreamEvent + 事件类型常量
│   │   ├── conversation.go         # 会话 CRUD 的 request/response DTO
│   │   ├── document.go             # 文档 CRUD 的 request/response DTO
│   │   └── envelope.go             # APIEnvelope + OK()/Err()
│   │
│   ├── graph/
│   │   └── agent.go                # NewAgentGraph() — ReAct 循环图 (START→ChatModel→Branch→Tools→loop→END)
│   │
│   ├── pipeline/
│   │   ├── rag.go                  # NewRAGChain() — Chain[string, *Message]: Retrieve→Template→ChatModel
│   │   └── document.go             # NewDocumentIngestionChain() — Chain[[]byte, []string]: Chunk→Embed→Index
│   │
│   ├── prompt/
│   │   ├── rag.go                  # NewRAGTemplate() — FString 模板
│   │   └── system.go               # systemRAG 常量 (system prompt)
│   │
│   ├── component/
│   │   ├── openaimodel/openai_chat_model.go  # OpenAIChatModel: Generate/Stream/BindTools
│   │   ├── openaiembed/openai_embedder.go    # OpenAIEmbedder: EmbedStrings (批处理)
│   │   ├── esindexer/elasticsearch_indexer.go # ElasticsearchIndexer: Store/ensureIndex
│   │   ├── esretriever/es.go                  # ESRRetriever: Retrieve (kNN 搜索)
│   │   └── tool/
│   │       ├── registry.go          # Tool 接口 + 全局 Registry (线程安全)
│   │       ├── datetime.go          # DateTimeTool: get_current_datetime
│   │       ├── web_search.go        # WebSearchTool: 占位符 (返回假数据)
│   │       └── rag.go               # RAGTool: retrieve_knowledge (包装 RAG chain)
│   │
│   ├── store/
│   │   ├── conversation.go          # ESConversationStore: ES 持久化的 Save/Create/List/Load/Delete (357行)
│   │   └── document.go              # DocumentStore: 内存 map (占位，待 ES 实现)
│   │
│   ├── callback/
│   │   └── logging.go               # LoggingCallback: Eino 组件生命周期日志
│   │
│   └── logger/
│       └── logger.go                # NewLogger: Zap + lumberjack 日志轮转
│
├── configs/
│   ├── config.yaml                  # 本地开发配置 (localhost, debug, Ollama qwen3.5:9b)
│   └── config.docker.yaml           # Docker 部署配置 (elasticsearch:9200, release, host.docker.internal)
│
├── Makefile                         # build/test/run/ollama/build-docker/up/down 等
├── Dockerfile                       # 多阶段 Go 构建 (golang:1.23-alpine → alpine:3.19)
├── docker-compose.yml               # 3 服务: elasticsearch + app + web(Nginx)
├── nginx.conf                       # SPA fallback + /api 代理 + SSE 支持
├── go.mod / go.sum                  # Go 依赖管理
└── server.exe                       # 本地编译的二进制 (不提交 git)
```

### 3.2 前端 React 代码 (`web/`)

```
web/
├── index.html                       # 中文本地化 HTML 入口
├── package.json                     # react 18, react-router-dom 6, lucide-react
├── vite.config.ts                   # Vite + Tailwind v4 插件 + /api 代理到 127.0.0.1:8080
├── tsconfig.json                    # strict, ES2020, bundler resolution
│
└── src/
    ├── main.tsx                     # ReactDOM.createRoot
    ├── App.tsx                      # RouterProvider
    ├── index.css                    # @import "tailwindcss" + blink 光标动画
    │
    ├── types/
    │   ├── chat.ts                  # ChatRequest, StreamEvent, Message, ToolCall, ChatStatus
    │   ├── conversation.ts          # ConversationItem, MessageItem
    │   ├── document.ts              # DocumentItem
    │   └── envelope.ts             # APIEnvelope<T>
    │
    ├── api/
    │   ├── client.ts               # apiFetch<T>() + rawFetch() + 自动 Bearer token
    │   ├── chat.ts                 # chatStream() — SSE 流式聊天 (fetch + ReadableStream)
    │   ├── conversation.ts         # 会话 CRUD (create/list/getMessages/delete)
    │   └── document.ts            # 文档上传/列表
    │
    ├── lib/
    │   └── sse-parser.ts           # parseSSEStream — 手动 UTF-8 SSE 解析器
    │
    ├── hooks/
    │   ├── useChatStream.ts        # 聊天流式状态机 (streamId 隔离、cancelled 标记)
    │   └── useConversations.ts     # 对话列表管理 + 消息加载
    │
    ├── components/
    │   ├── Layout.tsx              # 响应式侧边栏 + Outlet
    │   ├── Sidebar.tsx             # 对话列表面板
    │   ├── ChatBubble.tsx          # 消息气泡 (用户/助手 + 头像)
    │   ├── ChatInput.tsx           # 自动缩放文本输入 + 发送/取消
    │   ├── StreamRenderer.tsx      # 消息渲染 (连续消息分组 + 头像规则)
    │   ├── ToolCallBadge.tsx       # 工具调用状态 (pending/running/done/error)
    │   ├── ConversationCard.tsx    # 对话摘要按钮
    │   └── DocumentCard.tsx        # 文档卡片
    │
    ├── pages/
    │   ├── ChatPage.tsx            # 主聊天页面 (核心页面)
    │   ├── ConversationPage.tsx    # 对话管理页面
    │   └── DocumentPage.tsx        # 文档管理页面
    │
    └── router/
        └── index.tsx               # BrowserRouter: / → /chat, /chat, /conversations, /documents
```

---

## 四、核心数据流

### 4.1 聊天全链路

```
用户输入 → POST /api/v1/chat
  ↓
Gin: middleware.Recovery → CORS → Auth → Logger
  ↓
ChatHandler.Chat()
  ├─ 解析 ChatRequest {question, stream, agent, conversation_id}
  ├─ 无 convId → ConversationService.Create() 自动创建
  ├─ prependHistory() 注入历史消息到 prompt
  ├─ SaveUserMessage() (流开始前保存，防刷新丢失)
  │
  ├─ handleInvoke():   ChatService.Chat()     → RAG chain.Invoke()  → SSE 事件流
  ├─ handleStream():   ChatService.ChatStream() → RAG chain.Stream() → SSE 事件流
  └─ handleAgent():    ChatService.AgentStream() → Agent Graph.Stream() → SSE 事件流
      │
      ├─ SSE: data: {"type":"token","content":"..."}
      ├─ SSE: data: {"type":"tool_call","tool":"web_search"}
      ├─ SSE: data: {"type":"tool_result","content":"..."}
      ├─ SSE: data: {"type":"conversation_id","content":"conv_..."}
      └─ SSE: data: {"type":"done"}
  ↓
SaveAssistantMessage() (流结束后保存完整助手消息)
  ↓
前端: chatStream() → ReadableStream → parseSSEStream() → 事件驱动 UI
```

### 4.2 Agent ReAct 循环数据流

```
Agent Graph (compose.Graph[*schema.Message, *schema.Message])

START
  ↓
to_messages Lambda (单个 *Message → []*Message)
  ↓
ChatModel (已绑定 tools)
  ├── 返回值有 ToolCalls ──→ ToolsNode (执行工具)
  │                              ↓
  │                          loop_back Lambda (tools 输出 → chat_model 输入)
  │                              ↓
  │                          ChatModel (下一轮推理)
  │                              ├── 仍有 ToolCalls → 继续循环
  │                              └── 无 ToolCalls → END
  │
  └── 返回值无 ToolCalls ──→ END (最终输出)
```

### 4.3 SSE 前端消费流程

```
chatStream (api/chat.ts)
  → fetch POST (stream: true) → res.body.getReader()
  → parseSSEStream(reader)    // lib/sse-parser.ts
     → TextDecoder decode (stream: true, 防 UTF-8 截断)
     → buffer += newText
     → split by "\n\n"
     → 每行: trim "data: " → JSON.parse → yield StreamEvent

useChatStream (hooks/useChatStream.ts)
  ├─ streamIdRef 隔离 (递增 counter, 旧流事件静默忽略)
  ├─ 用户消息 → push to messages[]
  ├─ 空助手占位 → push to messages[]
  ├─ token     → append to 最后一条助手消息
  ├─ tool_call → status = tool_calling, 新增 ToolCall entries
  ├─ tool_result → mark last tool_call done
  ├─ conversation_id → callback 更新 URL
  ├─ done      → status = idle
  └─ error     → status = error, 显示错误
```

### 4.4 文档摄入流程

```
Upload POST /api/v1/documents (multipart form)
  ↓
DocumentHandler.UploadDocument()
  ↓
DocumentService.Ingest(data []byte)
  ↓
DocumentIngestionChain (compose.Chain[[]byte, []string])
  └─ Lambda Node:
     1. bytes → string
     2. chunkText() rune-aware 分块 (size=500, overlap=50)
     3. UUID 生成文档 ID
     4. embedder.EmbedStrings() 生成向量
     5. indexer.Store() → ES bulk index
     6. 返回文档 ID 列表
```

---

## 五、依赖注入图 (fx Module)

```
Config
 ├─→ Logger
 ├─→ ESClient → Indexer, Retriever, ESConversationStore
 ├─→ resolveModelProvider → ResolvedConfig{ChatModel,EmbeddingModel,BaseURL,APIKey,Provider}
 │    ├─→ ChatModel (OpenAIChatModel)
 │    └─→ Embedder (OpenAIEmbedder)
 │
 ├─ ChatModel + Retriever → RAGChain (compose.Chain[string, *schema.Message])
 ├─ ChatModel + RAGChain + tool.Registry → AgentGraph (compose.Graph[*Message, *Message])
 ├─ Embedder + Indexer → DocChain (compose.Chain[[]byte, []string])
 │
 ├─ RAGChain + AgentGraph + ConversationService → ChatService
 ├─ ESConversationStore → ConversationService
 ├─ DocChain → DocumentService
 │
 ├─ ChatService + ConversationService → ChatHandler
 ├─ ConversationService → ConversationHandler
 ├─ DocumentService → DocumentHandler
 │
 └─ Config + Logger + 3 Handlers → Router (*gin.Engine)
```

---

## 六、开发工作流（必须遵守）

### 6.1 「检查 → 测试 → 审计」三步法

每项功能开发必须严格执行：

```
第一步: CHECK (检查)
  ├─ 读取相关文件确认当前状态
  ├─ 分析数据结构/接口签名/数据流
  └─ 确认改动范围

第二步: TESTS (编写测试)
  ├─ 为新增功能编写单元测试
  ├─ 确保测试覆盖核心路径 + 边界情况
  └─ go test ./... -v -count=1

第三步: AUDIT (代码审查)
  ├─ 复查所有改动文件的差异
  ├─ 检查竞态条件/goroutine 安全
  ├─ 检查错误处理是否完备
  └─ 确认无调试代码残留
```

### 6.2 启动和调试命令

```bash
# 本地开发 (Go + ES)
cd D:\goagentpro
make ollama        # 启动 ES Docker + 本地运行 Go app
# 访问 http://127.0.0.1:8080/health

# 前端开发
cd D:\goagentpro\web
npm install
npm run dev        # Vite dev server, 代理 /api → 127.0.0.1:8080

# Docker 全栈
cd D:\goagentpro
# 先构建前端
cd web && npm run build && cd ..
# 再启动
docker compose up --build -d
# 访问 http://127.0.0.1:8081

# 测试
go test ./... -v -count=1

# 构建
go build -ldflags="-s -w" -o bin/goagent ./cmd/server
```

### 6.3 配置切换

| 场景 | 配置文件 | 关键差异 |
|------|---------|---------|
| 本地开发 | `configs/config.yaml` | `es.addresses: http://localhost:9200` |
| Docker 部署 | `configs/config.docker.yaml` | `es.addresses: http://elasticsearch:9200` |
| 环境变量 | 前缀 `GOAGENT_` | 例如 `GOAGENT_SERVER_PORT=9090` |

---

## 七、已知问题和改进方向 (TODO)

### P0 — 严重 (优先修复)

| # | 问题 | 位置 | 描述 |
|---|------|------|------|
| 1 | **WebSearchTool 占位符** | `internal/component/tool/web_search.go` | 仅返回假数据，需要接真实搜索 API (SerpAPI / Bing / 自定义) |
| 2 | **document.go handler stub** | `internal/handler/document.go` | `ListDocuments` 返回空列表，`DeleteDocument` 硬编码状态，需要 ES 实现 |
| 3 | **DocumentStore 内存实现** | `internal/store/document.go` | 用 `map[string]DocumentMeta`，不是 ES，重启后数据丢失 |
| 4 | **Auth 非强阻断** | `internal/server/middleware/auth.go` | 永不 reject 请求，仅标记用户为 anonymous/invalid_token |

### P1 — 重要

| # | 问题 | 位置 | 描述 |
|---|------|------|------|
| 5 | **handleAgent() 代码重复** | `internal/handler/chat.go` | agent 流和普通流的 handler 大量重复 (SSE 头、事件发送、持久化) |
| 6 | **history injection 丢失 tool call 上下文** | `internal/handler/chat.go` `prependHistory()` | 用字符串拼接历史，tool_calls 信息丢失 |
| 7 | **前端 token 硬编码** | `web/src/api/client.ts` | `Authorization: Bearer dev-token` 写死 |
| 8 | **前端无流式消息增量持久化** | `web/src/hooks/useChatStream.ts` | 流式消息逐 token 追加但不会实时保存，刷新时未完成的流丢失 |

### P2 — 改进

| # | 问题 | 位置 | 描述 |
|---|------|------|------|
| 9 | **无 loading/skeleton** | 前端 | 流式传输中只有闪烁光标，无骨架屏 |
| 10 | **文档类无 frontend 管理 UI** | 前端 `DocumentPage` | 仅有基本模板 |
| 11 | **无 ES index migration** | store 层 | ES mapping 硬编码在 `ensureIndex()` 中 |
| 12 | **OpenAIChatModel.BindTools 空实现** | `internal/component/openaimodel/openai_chat_model.go` | BindTools 是 no-op，tools 通过 graph 动态绑定 |

---

## 八、Agent 开发任务示例

### 8.1 修复 WebSearchTool（推荐第一个任务）

**目标**: 替换 `WebSearchTool.InvokableRun()` 的占位符，接入真实搜索 API。

**涉及文件**:
- `internal/component/tool/web_search.go` — 主要修改
- `internal/component/tool/registry.go` — 可能不需要改
- `internal/config/types.go` — 可能需要新增搜索 API 配置
- `internal/config/config.go` — 可能需要新增配置加载

**验收标准**:
1. `InvokableRun()` 返回真实搜索结果文本
2. 错误处理：搜索 API 不可达时返回友好错误
3. 测试覆盖：单元测试 mock 搜索 API

### 8.2 实现 Document ES 持久化

**目标**: 将 `DocumentStore` 从内存 map 迁移到 ES。

**涉及文件**:
- `internal/store/document.go` — 重写为 ES 实现
- `internal/config/types.go` — 可能需要新增 ES doc index 配置
- `internal/handler/document.go` — 修复 stub 逻辑
- `internal/server/fx.go` — 调整 Provide 依赖

### 8.3 添加新 Tool

**步骤**:
1. 在 `internal/component/tool/` 下新建 `xxx.go`
2. 实现 `tool.Tool` 接口 (Info + InvokableRun)
3. 在 `internal/server/fx.go` 的 `ProvideAgentGraph()` 中 `tool.Register(&YourTool{})`
4. 补充 `internal/component/tool/xxx_test.go`
5. 前端 `ToolCallBadge.tsx` 自动渲染 (无需修改)

---

## 九、用户偏好和沟通风格

| 维度 | 说明 |
|------|------|
| **语言** | 中文交流 |
| **风格** | 务实简洁、反过度设计 |
| **输出要求** | ✅/❌/🟡 表格化状态输出 + 代码级细节 |
| **回答格式** | 完整代码/数据流链路，不要摘要，要最完整的信息 |
| **问题模式** | 简短问题陈述 → 期待系统性诊断与修复 |
| **工作流** | 严格「检查 → 编写测试 → 审计」三步，每步强制验证后才进入下一阶段 |
| **迭代方式** | 多次迭代审查，逐步修正错误后生成最终版 |
| **视觉问题** | 通过截图描述，调试流程：报问题→截图→AI分析→验收→下一个 |

---

## 十、项目命名规范

| 类别 | 规范 |
|------|------|
| Go 文件 | `snake_case.go` |
| Go 类型 | `PascalCase` |
| Go 函数/方法 | `PascalCase` (exported), `camelCase` (unexported) |
| Go 常量 | `PascalCase` |
| Go 接口 | `{Name} + er` 后缀 (如 `Indexer`, `Retriever`) |
| Go 包名 | 小写单次/单数形式 |
| TypeScript 文件 | `camelCase.ts` |
| TypeScript 类型/接口 | `PascalCase` |
| TypeScript 函数 | `camelCase` |
| React 组件 | `PascalCase` |
| Docker 镜像名 | `goagentpro-{service}` (如 goagentpro-app) |
| ES 索引 | `goagent_*` (如 goagent_vectors, goagent_conversations) |
| 配置文件 | `config.{env}.yaml` |
| ENV 前缀 | `GOAGENT_` |

---

## 十一、Eino 关键 API 速查

```go
// Chain (线性流水线)
chain := compose.NewChain[string, *schema.Message]()
chain.AppendLambda(compose.InvokableLambda(...))
chain.AppendChatTemplate(template)
chain.AppendChatModel(cm)
runnable, _ := chain.Compile()

// Graph (有环图 - 自动 Pregel)
graph := compose.NewGraph[*schema.Message, *schema.Message]()
graph.AddChatModelNode("chat_model", cm)
graph.AddLambdaNode("tools", toolsNode)
graph.AddEdge(compose.START, "chat_model")
graph.AddBranch("chat_model", branch)  // 条件分支
graph.AddEdge("tools", "chat_model")   // 循环边
runnable, _ := graph.Compile()

// Stream (SSE 流式)
reader := runnable.Stream(ctx, input)
for {
    chunk, err := reader.Recv()
    if errors.Is(err, io.EOF) { break }
    // process chunk
}

// Tool definition
toolInfo := &schema.ToolInfo{
    Name: "my_tool",
    Description: "...",
    Params: schema.ParamsDesc{"type": "object", "properties": {...}},
}

// Tool 接口
type Tool interface {
    Info(ctx context.Context) (*schema.ToolInfo, error)
    InvokableRun(ctx context.Context, argumentsInJSON string, opts ...schema.ToolCallOption) (string, error)
}

// Callback
handler := callbacks.NewHandlerBuilder().
    OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
        log.Infof("Start: %s", info.Component)
        return ctx
    }).
    Build()
```

---

## 十二、重要文件路径速查

| 需要做的事情 | 要查看/修改的文件 |
|-------------|-----------------|
| 修改 LLM 调用逻辑 | `internal/component/openaimodel/openai_chat_model.go` |
| 修改 RAG 逻辑 | `internal/pipeline/rag.go` + `internal/prompt/rag.go` |
| 修改 Agent 循环 | `internal/graph/agent.go` + `internal/component/tool/*.go` |
| 新增 API 路由 | `internal/server/router.go` + 新 handler |
| 新增配置项 | `internal/config/types.go` + `configs/config.yaml` |
| 修改 SSE 事件格式 | `internal/model/chat.go` + `internal/handler/chat.go` |
| 修改消息渲染 | `web/src/components/StreamRenderer.tsx` + `ChatBubble.tsx` |
| 修改流式状态管理 | `web/src/hooks/useChatStream.ts` |
| 修改 API 调用 | `web/src/api/chat.ts` + `client.ts` |
| 修改 Docker 部署 | `docker-compose.yml` + `nginx.conf` + `Dockerfile` |
| 修改 ES 存储 | `internal/store/conversation.go` + `internal/component/esindexer/` + `esretriever/` |
| 调整依赖注入 | `internal/server/fx.go` |
| 添加 Prompt 模板 | `internal/prompt/` 下新建文件 |

---

## 十三、会话持久化细节（已完成的复杂修复）

当前会话持久化方案已完整实现并验证通过：

### 后端
- `store/conversation.go` (357行): ES 持久化的 Save/Create/List/Load/Delete
- 两个 ES index: `goagent_conversations` (元数据) + `goagent_conv_messages` (消息体)
- ES 连接重试: 15 次, 每次 2 秒间隔 (`NewESConversationStore`)
- 所有持久化使用 `context.Background()` 防请求上下文取消

### 前端
- `useChatStream.ts`: streamIdRef 隔离 + cancelled 标记防竞态
- `StreamRenderer.tsx`: 连续消息分组 (用户组头像在最后一条, 助手组头像在第一条)
- 间距规则: 同角色 `mt-0.5`, 切换角色 `mt-4`
- `useChatStream.sendMessage` 的 `useCallback` 依赖为 `[]`，通过 `statusRef.current` 读取状态

### 已知的持久化设计约束
- 用户消息: 流开始前保存 (防刷新丢失)
- 助手消息: 流结束后保存完整内容 (非增量保存)

---

## 十六、项目当前状态总结

| 维度 | 状态 | 备注 |
|------|------|------|
| RAG 流水线 | ✅ 完整实现 | Retrieve→Template→ChatModel |
| Agent ReAct 循环 | ✅ 完整实现 | ChatModel→(m)Tools→loop |
| 对话持久化 | ✅ 完整实现 | ES 双索引, 重试机制 |
| SSE 流式传输 | ✅ 完整实现 | token/tool_call/tool_result/done |
| 前端消息渲染 | ✅ 完整实现 | 消息分组, 头像规则, 光标动画 |
| 前端会话管理 | ✅ 完整实现 | CRUD, 列表, URL 同步 |
| 模型自动检测 | ✅ 完整实现 | Ollama probe → cloud fallback |
| CORS + Auth | ✅ 已实现但需加强 | Auth 非强阻断 |
| WebSearch 工具 | ❌ 占位符 | 需接入真实搜索 API |
| Document 持久化 | ❌ 内存占位 | 需迁移到 ES |
| Document handler API | 🟡 部分 stub | List/Delete 为假数据 |
| 前端文档管理 UI | 🟡 基础框架 | 缺失上传/删除等交互 |
| 测试覆盖 | 🟡 部分存在 | ChatService 有测试, 其他模块不足 |

---

> **给 Agent 的最后提示**: 
> 以上是 GoAgentPro 项目的完整上下文。你的任务是基于这个文档：
> 1. 先阅读项目中的实际代码验证文档描述
> 2. 按「检查→测试→审计」三步法执行任何修改
> 3. 输出时用 ✅/❌/🟡 表格 + 详细代码分析
> 4. 优先修复 P0 问题 (WebSearchTool, Document persistence)
> 5. 所有改动前先确认已有文件的最新状态
> 6. 启动前端后请用 `http://127.0.0.1:8081` (不是 localhost)
