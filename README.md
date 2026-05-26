GoAgentPro — 用 Go 写的 AI Agent 对话系统后端，核心是让 Agent 安全操控本地开发环境。

技术栈
语言：Go 1.25
AI 框架：CloudWeGo Eino v0.8.13
LLM 接入：OpenAI 协议兼容（OpenAI / Ollama / 阿里云百炼等）
向量存储：Elasticsearch 8.x（kNN + dense_vector）
Web：Gin + Swagger
DI：Uber Fx
配置：Viper
日志：Zap + Lumberjack
前端：Vite + React SPA（同仓）
定时任务：Robfig Cron
其他工具：uuid、validator、gonja（模板）、cors 中间件等
核心链路
Text
用户 → Gin API → Handler → Eino Agent Graph
├── Chat Model（LLM 多轮对话）
├── Tools（读写文件、执行命令等）
└── RAG（ES 混合检索 → LLM 增强回答）