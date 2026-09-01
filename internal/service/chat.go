package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/callback"
	"go.uber.org/zap"
)

// MessagePersister ChatService 对持久化层的最小依赖。
type MessagePersister interface {
	SaveMessages(ctx context.Context, convID string, msgs []*schema.Message) error
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
	reqQueue      *RequestQueue // 产品级排队系统
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
		logger:        logger,
	}
	// 启动调度 goroutine
	go svc.reqQueue.DrainAndDispatch(context.Background(), agentGraph)
	return svc
}

// Chat 走 RAG 链，回答完自动写 ES。
// 非流式分支接入语义缓存：命中直接返回（零 LLM 调用），未命中生成后写入。
// 注意：命中路径同样落库（会话历史必须完整），只是跳过 LLM/置信度/记忆提取。
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
		s.logger.Error("rag chain invoke failed, degraded response", zap.Error(err))
		return &schema.Message{
			Role:    schema.Assistant,
			Content: "⚠️ 知识库检索服务暂时不可用，本次回答未基于文档。请稍后重试，或直接描述你的问题。",
		}, nil
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
func (s *ChatService) ChatStream(ctx context.Context, question string) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.ragChain.Stream(ctx, question)
	if err != nil {
		s.logger.Error("rag chain stream failed, degraded response", zap.Error(err))
		degraded := &schema.Message{
			Role:    schema.Assistant,
			Content: "⚠️ 知识库检索服务暂时不可用，本次回答未基于文档。请稍后重试，或直接描述你的问题。",
		}
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

// SummarizeHistory 对超出 maxKeep 的旧对话做摘要压缩。
// 优先返回缓存；无缓存时异步生成，本次返回截断占位。
func (s *ChatService) SummarizeHistory(convID string, messages []*schema.Message, maxKeep int) string {
	if len(messages) <= maxKeep {
		return ""
	}

	// 有缓存直接用。
	if cached, ok := s.summaryCache.Load(convID); ok {
		return cached.(string)
	}

	oldMsgs := messages[:len(messages)-maxKeep]
	if len(oldMsgs) == 0 {
		return ""
	}

	// 异步生成摘要（不阻塞本次响应）。
	go s.generateSummaryAsync(convID, oldMsgs)

	// 本次退回简单截断提示。
	return fmt.Sprintf("[省略了 %d 条历史消息]", len(oldMsgs))
}

// generateSummaryAsync 后台生成摘要并缓存。
func (s *ChatService) generateSummaryAsync(convID string, oldMsgs []*schema.Message) {
	oldText := s.messagesToPlainText(oldMsgs)
	sysMsg := &schema.Message{
		Role:    schema.System,
		Content: "请用一段中文总结以下对话的关键信息和结论，不超过200字：",
	}
	userMsg := &schema.Message{Role: schema.User, Content: oldText}

	if s.memorySvc == nil || s.memorySvc.model == nil {
		return
	}

	resp, err := s.memorySvc.model.Generate(context.Background(), []*schema.Message{sysMsg, userMsg})
	if err != nil {
		s.logger.Warn("async summary generation failed", zap.Error(err))
		return
	}

	summary := "## 历史摘要\n" + resp.Content
	s.summaryCache.Store(convID, summary)
	s.logger.Debug("summary cached", zap.String("conv_id", convID))
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

func (s *ChatService) messagesToPlainText(messages []*schema.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		content := msg.Content
		if len(content) > 500 {
			content = content[:500] + "..."
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
