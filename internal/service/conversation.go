package service

import (
	"context"

	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"go.uber.org/zap"
)

// ConversationStore 是会话持久化的最小契约（consumer 定义，provider 实现）。
type ConversationStore interface {
	Create(ctx context.Context, id string, title string) error
	Exists(ctx context.Context, id string) (bool, error)
	List(ctx context.Context) ([]store.ConversationMeta, error)
	Load(ctx context.Context, conversationID string) ([]*schema.Message, error)
	Save(ctx context.Context, conversationID string, msgs []*schema.Message) error
	Delete(ctx context.Context, conversationID string) error
	UpdateTitle(ctx context.Context, id, title string) error
	DeleteAll(ctx context.Context) error
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

// Ensure 保证指定 ID 的会话存在：不存在则创建，存在直接复用。
// 用于客户端自带固定 ID 的场景（如股票页 stock_<code>）——此前这类 ID 没有
// 会话元数据，消息落了库但会话列表看不到、也无法回看。
// ID 格式由 handler 层校验，这里不做白名单。
func (s *ConversationService) Ensure(ctx context.Context, id, title string) error {
	exists, err := s.store.Exists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := s.store.Create(ctx, id, title); err != nil {
		return err
	}
	s.logger.Info("conversation ensured", zap.String("conv_id", id), zap.String("title", title))
	return nil
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

// Rename updates a conversation's title.
func (s *ConversationService) Rename(ctx context.Context, id, title string) error {
	return s.store.UpdateTitle(ctx, id, title)
}

// DeleteAll 清空全部会话及消息。
func (s *ConversationService) DeleteAll(ctx context.Context) error {
	return s.store.DeleteAll(ctx)
}
