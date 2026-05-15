# GoAgentPro

基于 [Eino](https://github.com/cloudwego/eino) 框架构建的智能 Agent 对话系统，支持 RAG 检索增强生成、工具调用和多轮对话。

## 架构

```
浏览器 (http://127.0.0.1:8081)
    │
    ▼
Nginx (8081 → 80) ─── 静态文件 + API 代理 ─── Go App (8080)
                                                │
                          ┌─────────────────────┼─────────────────────┐
                          │                     │                     │
                    RAG Pipeline          Agent Graph          Document Pipeline
                    Retrieve→Template     ChatModel→Tools       Chunk→Embed→Index
                       →ChatModel            ↕ loop
                          │                     │
                          └─────────┬───────────┘
                                    │
                              Elasticsearch
                         (goagent_vectors / conversations / documents)
```

## 技术栈

| 层 | 技术 |
|---|------|
| 后端 | Go 1.23 + Gin + CloudWeGo Eino v0.8.13 + uber-fx |
| 向量存储 | Elasticsearch 8.x (kNN) |
| 前端 | React 18 + TypeScript 5.8 + Vite 6 + Tailwind CSS 4 |
| 部署 | Docker Compose (ES + App + Nginx) |

## 快速开始

### 本地开发

```bash
# 1. 启动 ES（需 Docker）
docker run -d --name es -p 9200:9200 \
  -e "discovery.type=single-node" \
  -e "xpack.security.enabled=false" \
  docker.elastic.co/elasticsearch/elasticsearch:8.14.0

# 2. 启动后端
go run ./cmd/server

# 3. 启动前端（另一个终端）
cd web && npm i && npm run dev

# 4. 浏览器打开
# http://127.0.0.1:8081
```

### Docker 部署

```bash
# 构建前端
cd web && npm i && npm run build && cd ..

# 构建并启动所有服务
docker compose up -d --build
```

服务端口：Nginx 8081、App 8080、ES 9200

## 配置说明

主配置文件 `configs/config.yaml`：

```yaml
server:
  port: 8080
auth:
  api_key: ""           # 空 = 开发模式；设值后必须 Bearer Token
model_provider:
  mode: "auto"          # auto | local | cloud
  local:
    enabled: true       # 本地 Ollama
    chat_model: "qwen3.5:9b"
    embedding_model: "nomic-embed-text"
  cloud:
    enabled: false      # 云端 API（OpenAI 兼容）
    api_key: ""
search:
  enabled: false        # WebSearch tool，需 api_key
  base_url: "https://serpapi.com/search"
vector_store:
  elasticsearch:
    addresses: ["http://localhost:9200"]
```

## API 文档

### POST /api/v1/chat

```json
{ "question": "什么是Go语言?", "stream": true, "agent": false }
```

响应（非流式）：`{ "code": 0, "data": { "content": "...", "role": "assistant" } }`

响应（流式）：SSE 事件流，格式 `data: {"type":"token|done|error|tool_call|tool_result", "content":"..."}`

### GET/POST /api/v1/conversations
### GET /api/v1/conversations/:id/messages
### DELETE /api/v1/conversations/:id
### POST /api/v1/documents（multipart: file）
### GET /api/v1/documents
### DELETE /api/v1/documents/:id

## 目录结构

```
├── cmd/server/          # 入口
├── internal/
│   ├── config/          # 配置加载
│   ├── server/          # fx 依赖注入、路由、中间件
│   ├── handler/         # HTTP 处理器
│   ├── service/         # 业务服务层
│   ├── graph/           # Agent ReAct 图
│   ├── pipeline/        # RAG / Document 流水线
│   ├── component/       # Eino 组件实现
│   │   ├── tool/        # 工具（DateTime, WebSearch, RAG）
│   │   ├── openaimodel/ # OpenAI 兼容 ChatModel
│   │   ├── openaiembed/ # OpenAI 兼容 Embedder
│   │   ├── esindexer/   # ES 向量索引
│   │   └── esretriever/ # ES kNN 检索
│   ├── store/           # ES 持久化（会话、文档）
│   ├── model/           # API 数据模型
│   ├── prompt/          # Prompt 模板
│   └── logger/          # 日志
├── web/                 # React 前端
├── configs/             # YAML 配置文件
├── docker-compose.yml
├── Dockerfile
└── nginx.conf
```
