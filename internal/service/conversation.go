package service

import (
	"context"

	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"go.uber.org/zap"
)

// ConversationStore 是 ESConversationStore 的持久化接口
type ConversationStore interface {
	Create(ctx context.Context, id string, title string) error
	List(ctx context.Context) ([]store.ConversationMeta, error)
	Load(ctx context.Context, conversationID string) ([]*schema.Message, error)
	Save(ctx context.Context, conversationID string, msgs []*schema.Message) error
	Delete(ctx context.Context, conversationID string) error
}

// ConversationService wraps a ConversationStore.
type ConversationService struct {
	store  ConversationStore
	logger *zap.Logger
}

// NewConversationService —
func NewConversationService(s ConversationStore, logger *zap.Logger) *ConversationService {
	return &ConversationService{store: s, logger: logger}
}

// ConversationMeta is the public representation of a conversation.
type ConversationMeta struct {
	ID           string
	Title        string
	MessageCount int
	CreatedAt    string
	UpdatedAt    string
}

// Create 建新会话，返回 ID。
func (s *ConversationService) Create(ctx context.Context, title string) (string, error) {
	id := store.NewConversationID()
	if err := s.store.Create(ctx, id, title); err != nil {
		return "", err
	}
	return id, nil
}

// List 列全部会话，最新在前。
func (s *ConversationService) List(ctx context.Context) ([]ConversationMeta, error) {
	metas, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ConversationMeta, len(metas))
	for i, m := range metas {
		result[i] = ConversationMeta{
			ID:           m.ID,
			Title:        m.Title,
			MessageCount: m.MessageCount,
			CreatedAt:    m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:    m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return result, nil
}

// LoadMessages returns messages for a conversation.
func (s *ConversationService) LoadMessages(ctx context.Context, id string) ([]*schema.Message, error) {
	return s.store.Load(ctx, id)
}

// SaveMessages appends messages to a conversation.
func (s *ConversationService) SaveMessages(ctx context.Context, id string, msgs []*schema.Message) error {
	return s.store.Save(ctx, id, msgs)
}

// Delete deletes a conversation and its messages.
func (s *ConversationService) Delete(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}
