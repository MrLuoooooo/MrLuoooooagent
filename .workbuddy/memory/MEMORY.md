# GoAgent Pro — 项目约定与架构决策

## Agent 应用四系统闭环
- **记忆（Memory）**：ESMemoryStore 向量检索 + Supersede 乐观锁冲突解决
- **幻觉（Hallucination）**：FactCheck（工具结果对齐）+ CompareRAG（源文档比对）
- **评估（Evaluation）**：eval/ 包有标注测试框架，precision@k/recall@k 可用
- **置信度（Confidence）**：4 因子启发式打分 + SelfConsistency 多次采样
- **闭环**：postProcess 串联四系统 → Agent 输出 → 置信度 → 低分时幻觉检测 → 记忆提取

## 架构模式
- Eino 图/链：Lambda 节点 + ChatModel 节点 + ToolsNode，ReAct 循环
- 依赖注入：Uber Fx，Provider 函数注册
- 存储分层：model（API DTO）→ service（业务逻辑+接口定义）→ store（ES/文件实现）
- Consumer 定义 interface（service 层定义 MemoryStore/FeedbackStore），Provider 实现
- ES 模式：私有 esXxxDoc（含 embedding）↔ 公开 XxxMeta（不含存储细节）

## 文件组织
- 新增模块放 `internal/` 下对应目录，不建子目录
- 测试文件与源文件同目录，`_test.go` 后缀
- 配置统一在 `configs/config.yaml` + `config/types.go`
- 路由注册：`router.go` 的 v1 Group + `fx.go` 的 Module
