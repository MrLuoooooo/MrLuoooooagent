package service

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ChatService decouples HTTP handlers from Eino Runnable.
// It handles error wrapping, logging, and future cross-cutting concerns
// (rate limiting, auth, metrics) in one place.
type ChatService struct {
	ragChain   compose.Runnable[string, *schema.Message]
	agentGraph compose.Runnable[*schema.Message, *schema.Message]
	logger     *zap.Logger
}

// NewChatService creates a ChatService.
func NewChatService(
	ragChain compose.Runnable[string, *schema.Message],
	agentGraph compose.Runnable[*schema.Message, *schema.Message],
	logger *zap.Logger,
) *ChatService {
	return &ChatService{ragChain: ragChain, agentGraph: agentGraph, logger: logger}
}

// Chat invokes the RAG chain and returns the assistant message.
func (s *ChatService) Chat(ctx context.Context, question string) (*schema.Message, error) {
	msg, err := s.ragChain.Invoke(ctx, question)
	if err != nil {
		s.logger.Error("rag chain invoke failed", zap.Error(err))
		return nil, err
	}
	return msg, nil
}

// ChatStream returns a stream reader for RAG streaming.
func (s *ChatService) ChatStream(ctx context.Context, question string) (*schema.StreamReader[*schema.Message], error) {
	stream, err := s.ragChain.Stream(ctx, question)
	if err != nil {
		s.logger.Error("rag chain stream failed", zap.Error(err))
		return nil, err
	}
	return stream, nil
}

// Agent invokes the tool-calling agent graph.
func (s *ChatService) Agent(ctx context.Context, msg *schema.Message) (*schema.Message, error) {
	result, err := s.agentGraph.Invoke(ctx, msg)
	if err != nil {
		s.logger.Error("agent graph invoke failed", zap.Error(err))
		return nil, err
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
