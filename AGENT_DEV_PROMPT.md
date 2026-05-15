# GoAgentPro — 项目缺陷审计 + Agent 开发任务

> **生成时间**: 2026-05-13 08:41 CST  
> **用途**: 把下面这个文件直接复制给 agent，让它按 P0→P1→P2 顺序修

---

## 一、实话实说 —— 项目现状

**整体评价**: 核心骨架搭好了，但放到生产环境里漏洞百出。下面是一个一个过。

### ✅ 已经做好的（不用再折腾）

- RAG 检索问答流 (Retrieve→Template→ChatModel)
- Agent ReAct 循环 (ChatModel→Tools→loop)
- 对话持久化到 ES（双索引，含重试机制）
- SSE 流式传输（用户输入/流数据/结束信号全链路）
- 前端消息渲染（分组、头像、间距规则合理）
- 前端会话管理（CRUD + URL 同步）
- 模型自动检测（Ollama probe → cloud fallback）
- 基础 CI（Makefile、Docker Compose）

---

### ❌ 真正的问题 —— 按严重程度排列

---

#### 🚨 P0 — 不动不行的硬伤（4 个）

| # | 问题 | 严重程度 | 文件 | 具体表现 |
|---|------|---------|------|---------|
| 1 | **WebSearchTool 是个假工具** | 核心功能缺失 | `internal/component/tool/web_search.go:39` | `InvokableRun` 返回硬编码字符串 `"搜索结果占位: %q（需要接入搜索 API 后生效）"`，LLM 永远拿不到真实结果 |
| 2 | **DocumentStore 在内存里** | 重启丢数据 | `internal/store/document.go:14` | `map[string]DocumentMeta`，存不住，Docker 重启全丢 |
| 3 | **文档 List/Delete 是假接口** | 前端废了 | `internal/handler/document.go:56-67` | `ListDocuments` 返回 `Total: 0, Documents: []`；`DeleteDocument` 硬编码 `Status: "deleted"`，实际什么都没删 |
| 4 | **Auth 中间件不拦人** | 安全形同虚设 | `internal/server/middleware/auth.go:46` | 不管 token 对错，永远 `c.Next()`，`anonymous` 和 `authenticated` 一视同仁 |

#### ⚠️ P1 — 不修迟早出事（6 个）

| # | 问题 | 影响 | 文件 | 具体表现 |
|---|------|------|------|---------|
| 5 | **handleAgent 和 handleStream 代码高度重复** | 维护噩梦 | `internal/handler/chat.go:89-208` | 两套几乎完全相同的 SSE 处理逻辑（header flush→stream loop→persist），改一个忘了另一个 |
| 6 | **prependHistory 丢工具调用上下文** | Agent 多轮对话失忆 | `internal/handler/chat.go:218-227` | 字符串拼接 history，`schema.Message.ToolCalls` 字段全丢，Agent 第二轮就忘了上一步的结果 |
| 7 | **Agent 流不发射 tool_call/tool_result SSE 事件** | 前端看不到工具执行过程 | `internal/handler/chat.go:167-192` | `handleAgent` 的 stream 循环只发了 `token` 和 `done` 事件，`tool_call`/`tool_result` 类型虽然定义了但从没发过 |
| 8 | **前端 token 写死 'dev-token'** | 无法对接生产认证 | `web/src/api/client.ts:3` | `const TOKEN = 'dev-token'` 硬编码，无登录/配置机制 |
| 9 | **测试覆盖率极低** | 改代码全靠胆量 | 14 个 package 只有 4 个有测试文件（共 493 行） | 有测试的：`service/` `store/` `tool/` `callback/`。**零测试的**：`graph/` `pipeline/` `handler/` `component/openaimodel/` `component/openaiembed/` `component/esindexer/` `component/esretriever/` `config/` `logger/` `server/` |
| 10 | **README.md 是空的** | 新人进来一脸懵 | `README.md:1` | 内容 `"nothing......"` |

#### 🟡 P2 — 体验和运维层面的改进（8 个）

| # | 问题 | 类型 | 详情 |
|---|------|------|------|
| 11 | 前端流式传输无骨架屏/loading | 用户体验 | 只有闪烁光标，初始加载无反馈 |
| 12 | 前端文档管理页面是空壳 | 功能残缺 | `DocumentPage.tsx` 只显示 `DocumentCard` 模板 |
| 13 | Nginx 不等待 app 健康就转发请求 | 部署健壮性 | `depends_on` 无 `condition: service_healthy`，app 没起来时 Nginx 直接 502 |
| 14 | Docker Compose 无资源限制 | 运维 | 没有 `deploy.resources.limits`，ES 可能撑爆内存 |
| 15 | ES index mapping 硬编码在代码里 | 运维 | `ensureIndex()` 写死在 `esindexer` 中，改 mapping 要重新编译 |
| 16 | handleInvoke 没有持久化 | 数据一致性 | `handleInvoke` (非流式调用) 不保存消息，和流式行为的持久化逻辑不一致 |
| 17 | 对话列表为空时前端无友好提示 | 用户体验 | 空列表只有空白 |
| 18 | OpenAIChatModel.BindTools 空实现 | 架构迷惑 | 注释说 no-op，实际通过 graph 动态传 tools，但接口实现存在潜在混淆 |

---

## 二、Agent 开发指令（复制下面全部内容给你的 agent）

---

```
# GoAgentPro — Agent 开发任务书

## 你是谁
你是 GoAgentPro 项目的开发 Agent。你的工作是基于现有代码做增量开发。

## 项目位置
D:\goagentpro

## 技术栈
- 后端: Go 1.23 + Gin + CloudWeGo Eino v0.8.13 + Elasticsearch 8.x + uber-fx
- 前端: React 18 + TypeScript 5.8 + Vite 6 + Tailwind CSS 4 + lucide-react
- 部署: Docker Compose 三容器 (ES + Go App + Nginx)
- 禁止使用: Ant Design / MUI / Redux / Zustand / React Query / axios

## 必须遵守的约束
1. 前端访问必须用 http://127.0.0.1:8081，绝不用 localhost
2. Docker 环境下 ES 地址是 http://elasticsearch:9200
3. Eino 框架不要用 ADK 模式，直接用底层 compose.Graph/compose.Chain
4. 输出用 ✅/❌/🟡 表格 + 代码级详细分析，不要摘要
5. 每项改动前先「检查→编写测试→审计」三步走

## 执行顺序：P0 → P1 → P2

---

## 🚨 P0 — 必须立即修复

### 任务 1: 接入真实 WebSearch API

**目标**: 把 `WebSearchTool.InvokableRun()` 从占位符改成真实搜索。

**涉及文件**:
- `internal/component/tool/web_search.go` — 主要修改
- `internal/config/types.go` — 新增 SearchProvider 配置 (api_key, base_url, engine)
- `internal/config/config.go` — 新增配置加载
- `internal/config/config.yaml` + `config.docker.yaml` — 新增搜索配置节

**验收标准**:
1. 支持至少一种搜索 API（推荐 SerpAPI / 阿里云 OpenSearch / Bing Search）
2. 搜索结果以纯文本格式返回给 LLM（标题+摘要+URL）
3. API 不可达时返回友好错误，不 panic
4. 配置项写在 YAML 中，通过 viper 加载
5. 新配置项带默认值（disabled），默认不启用
6. 写 `internal/component/tool/web_search_test.go`，mock HTTP 客户端测试

---

### 任务 2: DocumentStore 从内存迁移到 ES

**目标**: DocumentStore 不再用 `map[string]DocumentMeta`，改用 ES 持久化。

**涉及文件**:
- `internal/store/document.go` — 重写为 ES 实现
- `internal/config/types.go` — 新增 `doc_index_name` 配置（默认 `goagent_documents`）
- `internal/handler/document.go` — 修复 ListDocuments/DeleteDocument 调用真实 store
- `internal/server/fx.go` — ProvideDocumentStore 改用 ES 版本

**验收标准**:
1. 支持 ES 的 index mapping：`document_id`(keyword), `filename`(text), `chunk_count`(integer), `created_at`(date), `content`(text)
2. UploadDocument 成功时同时写入 ES
3. ListDocuments 从 ES 读取
4. DeleteDocument 从 ES 按 ID 删除
5. 自动创建 index（参考 `conversation.go` 的模式）
6. 写 `internal/store/document_test.go`
7. 保存 chunks 到 goagent_vectors 时关联 document_id

---

### 任务 3: Auth 中间件增加阻断逻辑

**目标**: 让 Auth 中间件在配置了 APIKey 时拒绝未认证请求。

**涉及文件**:
- `internal/server/middleware/auth.go`

**验收标准**:
1. 如果 `cfg.Auth.APIKey` 不为空：无 token 或 token 不匹配 → `c.AbortWithStatusJSON(401, ...)`
2. 如果 `cfg.Auth.APIKey` 为空（开发模式）：行为不变（仅标记，不阻断）
3. 401 响应格式遵循 `model.APIEnvelope`：`{"code":401,"message":"unauthorized"}`
4. 写 `internal/server/middleware/auth_test.go`

---

## ⚠️ P1 — 必须修复

### 任务 4: handleAgent 和 handleStream 去重

**目标**: 抽离公共的 SSE 处理逻辑，消除两份重复代码。

**涉及文件**:
- `internal/handler/chat.go`

**方案**:
1. 抽取 `func (h *ChatHandler) streamSSE(c *gin.Context, convID string, reader *schema.StreamReader[*schema.Message], isAgent bool)`
2. 公共逻辑：SSE header flush、stream loop、convID event、done event、persist
3. agent 独有的：收集 tool_calls 用于 persist
4. 验证：handleStream 和 handleAgent 在 stream 分支都调用 `streamSSE`

**验收标准**:
1. 两份 SSE 处理逻辑合并为一
2. 非流式 agent 调用（handleAgent 的非 stream 分支）保持独立
3. 写 `internal/handler/chat_test.go`

---

### 任务 5: Agent 流发射 tool_call/tool_result SSE 事件

**目标**: 前端能在 Agent 模式下实时看到工具调用过程。

**涉及文件**:
- `internal/handler/chat.go` (handleAgent 的 stream 分支)
- 前端 `web/src/types/chat.ts` (可能不需要改，类型已定义)
- 前端 `web/src/hooks/useChatStream.ts` (确认处理逻辑)

**方案**:
在 `handleAgent` 的 stream loop 中：
1. 当 `len(chunk.ToolCalls) > 0` 时，每个 ToolCall 发射 `tool_call` 事件
2. 当工具执行完成（下一轮 ChatModel chunk），发射 `tool_result` 事件
3. 复用已有的 `model.StreamEvent{Type: model.EventToolCall}` 结构
4. 前端 `useChatStream` 已能处理这两种事件（tool_call→更新 toolCalls[], tool_result→标记 done），只需确认是否齐全

---

### 任务 6: prependHistory 携带 ToolCall 上下文

**目标**: Agent 多轮对话中保持工具调用上下文。

**涉及文件**:
- `internal/handler/chat.go` (prependHistory 函数)

**方案**:
1. `prependHistory` 参数从 `q string, history []*schema.Message` 改为 `q string, history []*schema.Message`
   （签名不变，改进实现）
2. 对于带 `ToolCalls` 的消息，格式化为可读文本（`Tool: xxx, Input: xxx, Result: xxx`）
3. 对于普通消息，保持现有格式

**验收标准**:
1. 包含 ToolCall 的消息能正确序列化到 prompt 上下文
2. 不会超过 token 限制（若 history 太长，截断最早的）
3. 写 `internal/handler/chat_test.go`

---

### 任务 7: 前端 token 接入配置机制

**目标**: 不再硬编码 `dev-token`。

**涉及文件**:
- `web/src/api/client.ts`

**方案**:
1. localStorage 存储 token：`localStorage.getItem('goagent_token') || 'dev-token'`
2. 提供 `setToken(token: string)` 导出函数
3. 前端无登录页面时不强制要求，但为未来对接做好准备

**验收标准**:
1. token 不再硬编码在源码中
2. `setToken('xxx')` 后后续请求自动使用新 token

---

### 任务 8: 补写核心测试

**目标**: 覆盖当前零测试的模块。

**优先级**:
1. `internal/graph/agent_test.go` — 测试图编译 + 分支逻辑
2. `internal/pipeline/rag_test.go` — 测试 RAG chain 编译 + template 渲染
3. `internal/pipeline/document_test.go` — 测试 chunkText 分块逻辑
4. `internal/component/openaimodel/openai_chat_model_test.go` — 测试 Generate/Stream
5. `internal/component/esindexer/elasticsearch_indexer_test.go` — 测试 ensureIndex
6. `internal/component/esretriever/es_test.go` — 测试 Retrieve

**每个测试要求**:
- 至少 2 个 passing test
- 覆盖正常路径 + 错误路径
- 使用 mock/fake 不要依赖外部 ES 或 LLM

---

### 任务 9: 补全 README.md

**目标**: README 不再写 "nothing......"

**内容要求**:
- 项目简介（一句话 + 架构图 ASCII）
- 技术栈清单
- 快速开始（本地开发 + Docker 部署）
- 配置说明（config.yaml 关键字段）
- API 文档（路径 + 请求/响应示例）
- 目录结构

---

## 🟡 P2 — 改进

### 任务 10: Docker Compose 增加 Nginx 启动依赖

在 `docker-compose.yml` 的 `web` 服务增加：等待 app 健康后再接受流量。

### 任务 11: handleInvoke 补充持久化

非流式调用 `handleInvoke` 也要保存用户和助手消息到 ES。

### 任务 12: Docker 容器加资源限制

在 `docker-compose.yml` 的每个服务加 `deploy.resources.limits`：
- ES: memory 1g
- app: memory 512m
- web: memory 128m

### 任务 13: 前端空列表友好提示

`ConversationPage` 和 `DocumentPage` 的列表为空时显示「暂无数据」文案。

---

## 通用开发规范

### 修改文件前必须做的
1. 先 `Read` 目标文件的完整内容
2. 理解现有代码的上下文和约束
3. 拟定改动方案后再下笔

### 每项改动后必须做的
1. `go test ./... -v -count=1`（后端）或 `npm run build`（前端）验证通过
2. 写 memory 记录改了什么、改了哪几个文件
3. 如果是 P0 任务，追加对应的测试文件

### 遇到问题时的处理顺序
1. 先读文件确认问题描述是否准确
2. 搜索类似实现参考（比如 DocumentStore 参考 ConversationStore 的 ES 模式）
3. 跨文件改造时列出所有受影响文件清单
4. 需要新增配置项时同步修改 `types.go` `config.go` 和两个 YAML

---

**核心准则**: 代码诚实、不准造假、每个改动必须有对应的测试或验证手段。
```

---

## 三、文件清单（方便 agent 一次性加载）

把下面这些文件传给 agent，它能直接开工：

```
需要读取的源代码（按任务分组）:

任务1(WebSearch):
  D:\goagentpro\internal\component\tool\web_search.go
  D:\goagentpro\internal\component\tool\registry.go
  D:\goagentpro\internal\component\tool\datetime.go (参考实现)
  D:\goagentpro\internal\config\types.go
  D:\goagentpro\internal\config\config.go
  D:\goagentpro\configs\config.yaml
  D:\goagentpro\configs\config.docker.yaml

任务2(Document ES):
  D:\goagentpro\internal\store\document.go
  D:\goagentpro\internal\store\conversation.go (参考实现)
  D:\goagentpro\internal\handler\document.go
  D:\goagentpro\internal\server\fx.go

任务3(Auth):
  D:\goagentpro\internal\server\middleware\auth.go

任务4+5+6(Chat Handler):
  D:\goagentpro\internal\handler\chat.go
  D:\goagentpro\internal\service\chat.go
  D:\goagentpro\internal\model\chat.go

任务7(Frontend Token):
  D:\goagentpro\web\src\api\client.ts

任务8(Tests):
  D:\goagentpro\internal\graph\agent.go
  D:\goagentpro\internal\pipeline\rag.go
  D:\goagentpro\internal\pipeline\document.go
  D:\goagentpro\internal\component\esindexer\elasticsearch_indexer.go
  D:\goagentpro\internal\component\esretriever\es.go

任务9(README):
  D:\goagentpro\README.md

任务10-13(Docker等):
  D:\goagentpro\docker-compose.yml
  D:\goagentpro\nginx.conf
```

---

> **一句话总结**: P0 的 4 个问题是真硬伤 —— WebSearch 假工具、Document 存内存、List/Delete 假接口、Auth 不拦人。P1 的 6 个是架构债务 —— 代码重复、丢上下文、没测试。先把 P0 干完，项目才能说「能用」。
