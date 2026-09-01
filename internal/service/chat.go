package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/callback"
	"go.uber.org/zap"
)

// MessagePersister ChatService 对持久化层的最小依赖。
type MessagePersister interface {
	SaveMessages(ctx context.Context, convID string, msgs []*schema.Message) error
}

// CheckpointCleaner ChatService 对断点存储的最小依赖（消费方定义接口）。
// store.CheckpointStore 天然满足；Agent 执行完毕清理由 service 编排，
// handler 不直接持有存储层。
type CheckpointCleaner interface {
	Delete(ctx context.Context, checkPointID string) error
}

// ChatService 把 handler 和 Eino 图/链解耦，管消息持久化 + 记忆提取 + 置信度 + 幻觉检测。
type ChatService struct {
	ragChain      compose.Runnable[string, *schema.Message]
	agentGraph    compose.Runnable[*schema.Message, *schema.Message]
	persister     MessagePersister
	memorySvc     *MemoryService
	feedbackSvc   *FeedbackService
	confidenceSvc *ConfidenceService
	semanticCache *SemanticCache // 可为 nil，命中时跳过 LLM
	summaryCache  sync.Map
	reqQueue      *RequestQueue  // 产品级排队系统
	contextWindow int            // 当前模型上下文窗口（token），短期记忆预算的来源
	cpStore       CheckpointCleaner // 可为 nil；Agent 完成后清理断点
	logger        *zap.Logger
}

// NewChatService —
func NewChatService(
	ragChain compose.Runnable[string, *schema.Message],
	agentGraph compose.Runnable[*schema.Message, *schema.Message],
	persister MessagePersister,
	memorySvc *MemoryService,
	feedbackSvc *FeedbackService,
	confidenceSvc *ConfidenceService,
	semanticCache *SemanticCache,
	contextWindow int,
	cpStore CheckpointCleaner,
	logger *zap.Logger,
) *ChatService {
	svc := &ChatService{
		ragChain:      ragChain,
		agentGraph:    agentGraph,
		persister:     persister,
		memorySvc:     memorySvc,
		feedbackSvc:   feedbackSvc,
		confidenceSvc: confidenceSvc,
		semanticCache: semanticCache,
		reqQueue:      NewRequestQueue(logger),
		contextWindow: contextWindow,
		cpStore:       cpStore,
		logger:        logger,
	}
	// 启动调度 goroutine
	go svc.reqQueue.DrainAndDispatch(context.Background(), agentGraph)
	return svc
}

// CleanupCheckpoint Agent 执行完毕后清理该会话的断点文件。
// 断点只服务于中途恢复，回答完整产出后即失去价值，残留只会白占磁盘。
// 清理失败仅记 WARN：下次同会话 Set 会覆盖旧断点，不影响功能。
func (s *ChatService) CleanupCheckpoint(ctx context.Context, convID string) {
	if s.cpStore == nil {
		return
	}
	if err := s.cpStore.Delete(ctx, convID); err != nil {
		s.logger.Warn("cleanup checkpoint",
			zap.String("conv_id", convID), zap.Error(err))
	}
}

// ragDegradedNotice RAG 检索链不可用时的统一降级文案。
// 单点维护：非流式（Chat）与流式（ChatStream）共用，改文案只动这里。
const ragDegradedNotice = "⚠️ 知识库检索服务暂时不可用，本次回答未基于文档。请稍后重试，或直接描述你的问题。"

// Chat 走 RAG 链，回答完自动写 ES。
// 非流式分支接入语义缓存：命中直接返回（零 LLM 调用），未命中生成后写入。
// 注意：命中/降级路径同样落库（会话历史必须完整），只是跳过 LLM/置信度/记忆提取。
func (s *ChatService) Chat(ctx context.Context, question string, convID string) (*schema.Message, error) {
	if s.semanticCache != nil {
		if ans, hit := s.semanticCache.Get(ctx, question); hit {
			hits, _ := s.semanticCache.Stats()
			s.logger.Info("semantic cache hit",
				zap.String("question", question),
				zap.Int64("total_hits", hits),
			)
			msg := &schema.Message{Role: schema.Assistant, Content: ans}
			if err := s.SaveAssistantMessage(convID, msg.Content, nil); err != nil {
				s.logger.Error("persist cached assistant message", zap.String("conv_id", convID), zap.Error(err))
			}
			return msg, nil
		}
	}
	msg, err := s.ragChain.Invoke(ctx, question)
	if err != nil {
		// 降级：检索组件（ES/MySQL）不可用时不 500，回一段兜底消息。
		// 日志保留完整错误供排查；产品化升级点是这里换成纯 LLM 直接回答。
		s.logger.Error("rag chain invoke failed, degraded response",
			zap.String("conv_id", convID),
			zap.String("question", question),
			zap.Error(err))
		degraded := &schema.Message{Role: schema.Assistant, Content: ragDegradedNotice}
		// 降级回复同样必须落库：否则刷新会话历史时这条回复凭空消失，
		// 与"会话历史必须完整"契约矛盾（流式路径由 handler 落库，非流式在此落）。
		if err := s.SaveAssistantMessage(convID, degraded.Content, nil); err != nil {
			s.logger.Error("persist degraded assistant message",
				zap.String("conv_id", convID),
				zap.Error(err))
		}
		return degraded, nil
	}
	if s.semanticCache != nil {
		s.semanticCache.Put(ctx, question, msg.Content)
	}
	if err := s.SaveAssistantMessage(convID, msg.Content, nil); err != nil {
		s.logger.Error("persist assistant message", zap.String("conv_id", convID), zap.Error(err))
	}
	return msg, nil
}

// ChatStream 返回 RAG 流。
// 降级时不落库（无 convID 参数）：流式路径的落库职责在 handler（chat.go 收流后统一 SaveMessages）。
func (s *ChatService) ChatStream(ctx context.Context, question string) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.ragChain.Stream(ctx, question)
	if err != nil {
		s.logger.Error("rag chain stream failed, degraded response",
			zap.String("question", question),
			zap.Error(err))
		degraded := &schema.Message{Role: schema.Assistant, Content: ragDegradedNotice}
		return schema.StreamReaderFromArray([]*schema.Message{degraded}), nil
	}
	return stream, nil
}

// SaveUserMessage 先落用户消息。
func (s *ChatService) SaveUserMessage(convID, question string) error {
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{
		{Role: schema.User, Content: question},
	})
}

// SaveAssistantMessage 只写助手回复。
func (s *ChatService) SaveAssistantMessage(convID, answer string, toolCalls []schema.ToolCall) error {
	msg := &schema.Message{Role: schema.Assistant, Content: answer}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{msg})
}

// SaveMessages 写一对用户+助手消息。
func (s *ChatService) SaveMessages(ctx context.Context, convID, question, answer string, toolCalls []schema.ToolCall) error {
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{
		{Role: schema.User, Content: question},
		{Role: schema.Assistant, Content: answer, ToolCalls: toolCalls},
	})
}

// Agent 跑 Agent 图 → 持久化 → 后处理（置信度 + 记忆提取 + 幻觉校验）。
func (s *ChatService) Agent(ctx context.Context, msg *schema.Message, convID string, opts ...compose.Option) (*schema.Message, error) {
	// 工具结果收集器：callback 写，Agent 执行完后读，传给置信度评估做 FactCheck 对齐
	bag := &callback.ToolResultsBag{}
	collector := callback.NewToolCollector(bag)
	allOpts := append([]compose.Option{compose.WithCallbacks(collector)}, opts...)

	result, err := s.agentGraph.Invoke(ctx, msg, allOpts...)
	if err != nil {
		s.logger.Error("agent graph invoke failed", zap.Error(err))
		return nil, err
	}

	// 先持久化纯净结果（不带置信度标注）。
	if err := s.SaveAssistantMessage(convID, result.Content, result.ToolCalls); err != nil {
		s.logger.Error("persist agent assistant message", zap.String("conv_id", convID), zap.Error(err))
	}

	// 工具结果传入置信度评估，实现真正的 FactCheck 对齐
	result = s.postProcess(ctx, msg, result, convID, bag.Results)

	return result, nil
}

// AgentStream 返回 Agent 图流。
func (s *ChatService) AgentStream(ctx context.Context, msg *schema.Message, opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.agentGraph.Stream(ctx, msg, opts...)
	if err != nil {
		s.logger.Error("agent graph stream failed", zap.Error(err))
		return nil, err
	}
	return stream, nil
}

// QueueSubmit 提交请求到产品级排队系统，返回结果通道。
// 永远不拒绝——入队即返回排队状态。
func (s *ChatService) QueueSubmit(req *QueuedRequest) *QueueResult {
	return s.reqQueue.Submit(req)
}

// PendingCount 当前排队人数。
func (s *ChatService) PendingCount() int {
	return s.reqQueue.PendingCount()
}

// —— 短期记忆 token 预算裁剪（对标 CC autoCompact，文档 §3）——

// reservedOutputTokens 给模型回复预留的输出预算。
const reservedOutputTokens = 8 * 1024

// minTokenBudget 预算下限：窗口再小也至少保留这么多，防止极端配置下历史全被裁光。
const minTokenBudget = 4 * 1024

// EstimateTokens 估算文本 token 数（纯函数）。
// 中文约 1.5-2 字符/token，英文约 4 字符/token，混合文本按 1.6 字符/token 近似。
// 先近似后校准（文档 §7），不为精确 tokenizer 阻塞进度。
func EstimateTokens(s string) int {
	runes := utf8.RuneCountInString(s)
	if runes == 0 {
		return 0
	}
	t := runes * 10 / 16 // ≈ runes / 1.6
	if t == 0 {
		t = 1
	}
	return t
}

// trimHistoryByToken 在 token 预算内保留"前缀摘要 + 最近消息"（纯函数，可单测）。
// messages 必须按时间升序；从尾部向前累计 token，放不下的旧消息交给 summarize
// 生成结构化摘要并注入头部（摘要本身计入语义而非预算——它必然远小于被裁内容）。
// 预算内放得下时原样返回，不触发 summarize。
func trimHistoryByToken(messages []*schema.Message, budget int, summarize func([]*schema.Message) string) []*schema.Message {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content)
	}
	if total <= budget || len(messages) == 0 {
		return messages
	}

	// 从尾部向前找能整段放下的最近消息窗口
	acc := 0
	keep := len(messages) // [keep:] 保留
	for i := len(messages) - 1; i >= 0; i-- {
		t := EstimateTokens(messages[i].Content)
		if acc+t > budget {
			break
		}
		acc += t
		keep = i
	}
	// 兜底：预算内连最后一条都放不下（如超长工具结果撑爆单条），
	// 全部交给摘要——绝不原样返回超预算历史让模型上下文爆炸。
	if keep >= len(messages) && summarize == nil {
		return messages
	}

	trimmed := make([]*schema.Message, 0, len(messages)-keep+1)
	if summarize != nil {
		if summary := summarize(messages[:keep]); summary != "" {
			trimmed = append(trimmed, &schema.Message{Role: schema.User, Content: summary})
		}
	}
	trimmed = append(trimmed, messages[keep:]...)
	return trimmed
}

// TrimHistory 在当前模型的 token 预算内裁剪会话历史。
// 摘要是同步生成的（LLM 一次调用，命中缓存直接复用）——绝不返回占位符，
// 不让模型在失忆状态下作答。落库不受影响：裁剪只影响发给模型的上下文。
func (s *ChatService) TrimHistory(convID string, history []*schema.Message) []*schema.Message {
	if len(history) == 0 {
		return history
	}
	budget := s.contextWindow - reservedOutputTokens
	if budget < minTokenBudget {
		budget = minTokenBudget
	}

	trimmed := trimHistoryByToken(history, budget, func(old []*schema.Message) string {
		return s.summarizeSync(convID, old)
	})
	if len(trimmed) != len(history) {
		origTokens, trimTokens := 0, 0
		for _, m := range history {
			origTokens += EstimateTokens(m.Content)
		}
		for _, m := range trimmed {
			trimTokens += EstimateTokens(m.Content)
		}
		s.logger.Info("history trimmed by token budget",
			zap.String("conv_id", convID),
			zap.Int("orig_msgs", len(history)),
			zap.Int("trimmed_msgs", len(trimmed)),
			zap.Int("orig_tokens", origTokens),
			zap.Int("trimmed_tokens", trimTokens),
			zap.Int("budget", budget),
		)
	}
	return trimmed
}

// summarizeSync 同步生成结构化摘要（对标 CC compact prompt 四段精简版）。
// 命中缓存直接返回；未命中调用 LLM，失败 WARN 后返回空串——宁可没有摘要，
// 也不给失忆模型塞占位符。缓存键 convID:被裁消息数，裁剪边界变化自动失效。
func (s *ChatService) summarizeSync(convID string, oldMsgs []*schema.Message) string {
	if len(oldMsgs) == 0 {
		return ""
	}
	cacheKey := fmt.Sprintf("%s:%d", convID, len(oldMsgs))
	if cached, ok := s.summaryCache.Load(cacheKey); ok {
		if str, ok := cached.(string); ok {
			return str
		}
	}
	if s.memorySvc == nil || s.memorySvc.model == nil {
		return ""
	}

	sysMsg := &schema.Message{
		Role: schema.System,
		Content: `你是对话摘要器。把这段被裁剪的对话历史压缩成结构化摘要，供 AI 在后续对话中保持上下文连续性。严格按以下四段输出，用 markdown：
## 目标与约束
用户在这段对话中要完成什么，有什么明确要求或限制。
## 关键事实与代码
涉及的重要文件、数据、结论；代码片段和 file:line 引用必须原文保留（verbatim），禁止转述。
## 当前进度
这段对话结束推进到了哪一步，最近一次操作的状态。
## 下一步
接下来要做什么，严格对应当前任务的延续，禁止发散到旧任务。

总长度不超过 600 字（代码引用不计入）。只输出摘要本身。`,
	}
	userMsg := &schema.Message{Role: schema.User, Content: s.messagesToPlainText(oldMsgs)}

	resp, err := s.memorySvc.model.Generate(context.Background(), []*schema.Message{sysMsg, userMsg})
	if err != nil {
		s.logger.Warn("sync summary generation failed, trimming without summary",
			zap.String("conv_id", convID),
			zap.Int("dropped_msgs", len(oldMsgs)),
			zap.Error(err))
		return ""
	}
	summary := resp.Content
	s.summaryCache.Store(cacheKey, summary)
	return summary
}

// ---- 闭环后处理 ----

// postProcess 执行 Agent 响应的后处理流水线：
// 1. 置信度评估 → 2. 幻觉检测 → 3. 低置信度标记 → 4. 异步提取长期记忆。
// toolResults 来自 Agent 执行过程中收集的实际工具返回，用于置信度的工具对齐检测。
// 返回可能被标记的 result（原始 content 已保存到 ES，此处只影响用户看到的输出）。
func (s *ChatService) postProcess(
	ctx context.Context,
	userMsg *schema.Message,
	assistantMsg *schema.Message,
	convID string,
	toolResults []*schema.Message,
) *schema.Message {
	// Step 1: 置信度评估（含工具结果对齐）
	if s.confidenceSvc != nil {
		cr := s.confidenceSvc.Evaluate(ctx, assistantMsg.Content, toolResults, nil)
		if cr.NeedVerify {
			s.logger.Warn("low confidence response",
				zap.Float64("score", cr.Score),
				zap.String("conv_id", convID),
				zap.Strings("warnings", cr.Warnings),
			)

			// Step 2: 低置信度时尝试做事实校验。
			if s.feedbackSvc != nil {
				factual, conflicts := s.feedbackSvc.FactCheck(toolResults, assistantMsg.Content)
				if !factual {
					cr.Warnings = append(cr.Warnings, conflicts...)
				}
			}

			// Step 3: 在回复前注入置信度标注。
			note := s.confidenceSvc.InjectConfidenceNote(cr)
			if note != "" {
				// 创建新的 Message，不改原消息（ES 已存纯净版）。
				marked := *assistantMsg
				marked.Content = note + assistantMsg.Content
				assistantMsg = &marked
			}
		}
	}

	// Step 4: 异步提取长期记忆（含工具结果，不阻塞响应）。
	s.extractMemories(ctx, convID, userMsg, assistantMsg, toolResults)

	return assistantMsg
}

// ---- 内部方法 ----

func (s *ChatService) extractMemories(ctx context.Context, convID string, userMsg, assistantMsg *schema.Message, toolResults []*schema.Message) {
	if s.memorySvc == nil {
		return
	}
	// 包含工具结果，让 LLM 提取时看到原始数据（如股票 API 返回的行情）
	messages := make([]*schema.Message, 0, 2+len(toolResults))
	messages = append(messages, userMsg)
	messages = append(messages, toolResults...) // 工具结果在 assistant 之前
	messages = append(messages, assistantMsg)
	s.memorySvc.ExtractAndStore(ctx, "default", convID, messages)
}

// messagesToPlainText 拼接消息为摘要输入文本。
// 工具结果截断到 1500 字符——摘要 prompt 要求代码 verbatim，200 字符会把
// 关键代码截没，摘要等于没做。
func (s *ChatService) messagesToPlainText(messages []*schema.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		content := msg.Content
		if len(content) > 1500 {
			content = content[:1500] + "..."
		}
		if content == "" && len(msg.ToolCalls) > 0 {
			names := make([]string, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				names[i] = tc.Function.Name
			}
			content = "调用了工具: " + strings.Join(names, ", ")
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", string(msg.Role), content))
	}
	return sb.String()
}
