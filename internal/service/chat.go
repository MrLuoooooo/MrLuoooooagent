package service

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// MessagePersister 是 ChatService 对持久化层的最小依赖。
// 用接口而不是直接引用 ConversationService，方便测试 mock。
type MessagePersister interface {
	SaveMessages(ctx context.Context, convID string, msgs []*schema.Message) error
}

// ChatService 把 handler 和 Eino 图/链解耦，顺带管消息持久化。
type ChatService struct {
	ragChain   compose.Runnable[string, *schema.Message]
	agentGraph compose.Runnable[*schema.Message, *schema.Message]
	persister   MessagePersister
	logger     *zap.Logger
}


// NewChatService —
func NewChatService(
	ragChain compose.Runnable[string, *schema.Message],
	agentGraph compose.Runnable[*schema.Message, *schema.Message],
	persister MessagePersister,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{ragChain: ragChain, agentGraph: agentGraph, persister: persister, logger: logger}
}

// Chat 走 RAG 链，回答完自动写 ES。
// handler 调用前要自己 SaveUserMessage，保证原始问题先落库。
func (s *ChatService) Chat(ctx context.Context, question string, convID string) (*schema.Message, error) {
	msg, err := s.ragChain.Invoke(ctx, question)
	if err != nil {
		s.logger.Error("rag chain invoke failed", zap.Error(err))
		return nil, err
	}

	// Persist assistant message only (user message was saved by handler).
	if err := s.SaveAssistantMessage(convID, msg.Content, nil); err != nil {
		s.logger.Error("persist assistant message", zap.String("conv_id", convID), zap.Error(err))
	}

	return msg, nil
}

// ChatStream 返回 RAG 流。handler 自己收流、拼完整后再调 SaveMessages。
func (s *ChatService) ChatStream(ctx context.Context, question string) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.ragChain.Stream(ctx, question)
	if err != nil {
		s.logger.Error("rag chain stream failed", zap.Error(err))
		return nil, err
	}
	return stream, nil
}

// SaveUserMessage 先落用户消息（流还没开始时写），防止页面刷新丢数据。
func (s *ChatService) SaveUserMessage(convID, question string) error {
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{
		{Role: schema.User, Content: question},
	})
}

// SaveAssistantMessage 只写助手回复，不重复写用户消息。
func (s *ChatService) SaveAssistantMessage(convID, answer string, toolCalls []schema.ToolCall) error {
	msg := &schema.Message{Role: schema.Assistant, Content: answer}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{msg})
}

// SaveMessages 写一对用户+助手消息。内部用 background context 防 SSE 流结束后被取消。
func (s *ChatService) SaveMessages(ctx context.Context, convID, question, answer string, toolCalls []schema.ToolCall) error {
	// Use background context for the actual ES write to avoid cancellation issues
	// (SSE stream may have ended, cancelling the HTTP request context)
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{
		{Role: schema.User, Content: question},
		{Role: schema.Assistant, Content: answer, ToolCalls: toolCalls},
	})
}

// Agent 跑 Agent 图（ReAct 循环），结果自动写 ES。
func (s *ChatService) Agent(ctx context.Context, msg *schema.Message, convID string) (*schema.Message, error) {
	result, err := s.agentGraph.Invoke(ctx, msg)
	if err != nil {
		s.logger.Error("agent graph invoke failed", zap.Error(err))
		return nil, err
	}
	if err := s.SaveAssistantMessage(convID, result.Content, result.ToolCalls); err != nil {
		s.logger.Error("persist agent assistant message", zap.String("conv_id", convID), zap.Error(err))
	}
	return result, nil
}

// AgentStream 返回 Agent 图流。
func (s *ChatService) AgentStream(ctx context.Context, msg *schema.Message) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.agentGraph.Stream(ctx, msg)
	if err != nil {
		s.logger.Error("agent graph stream failed", zap.Error(err))
		return nil, err
	}
	return stream, nil
}

