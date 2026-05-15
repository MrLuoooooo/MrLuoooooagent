package service

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// MessagePersister is the minimal persistence interface ChatService needs.
// Using an interface here instead of *ConversationService directly means
// tests can provide a simple mock without importing the store package.
type MessagePersister interface {
	SaveMessages(ctx context.Context, convID string, msgs []*schema.Message) error
}

// ChatService decouples HTTP handlers from Eino Runnable.
// It also owns message persistence via MessagePersister.
type ChatService struct {
	ragChain   compose.Runnable[string, *schema.Message]
	agentGraph compose.Runnable[*schema.Message, *schema.Message]
	persister   MessagePersister
	logger     *zap.Logger
}


// NewChatService creates a ChatService.
func NewChatService(
	ragChain compose.Runnable[string, *schema.Message],
	agentGraph compose.Runnable[*schema.Message, *schema.Message],
	persister MessagePersister,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{ragChain: ragChain, agentGraph: agentGraph, persister: persister, logger: logger}
}

// Chat invokes the RAG chain, persists the assistant message, and returns it.
// User message must be saved by the handler before calling this (to keep original question).
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

// ChatStream returns a stream reader for RAG streaming.
// The caller is responsible for collecting the full response and calling SaveMessages.
// We expose the convSvc so the handler can persist after stream ends.
func (s *ChatService) ChatStream(ctx context.Context, question string) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.ragChain.Stream(ctx, question)
	if err != nil {
		s.logger.Error("rag chain stream failed", zap.Error(err))
		return nil, err
	}
	return stream, nil
}

// SaveUserMessage persists a single user message immediately (before streaming starts).
// This ensures the user's question is not lost if the page is refreshed mid-stream.
// Uses context.Background() to avoid HTTP request cancellation races.
func (s *ChatService) SaveUserMessage(convID, question string) error {
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{
		{Role: schema.User, Content: question},
	})
}

// SaveAssistantMessage persists only the assistant response (user was already saved).
// This avoids duplicating the user message in ES when called after SaveUserMessage.
func (s *ChatService) SaveAssistantMessage(convID, answer string, toolCalls []schema.ToolCall) error {
	msg := &schema.Message{Role: schema.Assistant, Content: answer}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{msg})
}

// SaveMessages persists a user-assistant message pair. Uses context.Background() internally
// so that the write is not cancelled if the HTTP request context is already done.
func (s *ChatService) SaveMessages(ctx context.Context, convID, question, answer string, toolCalls []schema.ToolCall) error {
	// Use background context for the actual ES write to avoid cancellation issues
	// (SSE stream may have ended, cancelling the HTTP request context)
	return s.persister.SaveMessages(context.Background(), convID, []*schema.Message{
		{Role: schema.User, Content: question},
		{Role: schema.Assistant, Content: answer, ToolCalls: toolCalls},
	})
}

// Agent invokes the tool-calling agent graph and persists the assistant message.
// User message must be saved by the handler before calling this.
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

// AgentStream returns a stream reader for the agent graph.
func (s *ChatService) AgentStream(ctx context.Context, msg *schema.Message) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.agentGraph.Stream(ctx, msg)
	if err != nil {
		s.logger.Error("agent graph stream failed", zap.Error(err))
		return nil, err
	}
	return stream, nil
}

