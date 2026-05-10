package service

import (
	"context"

	"github.com/cloudwego/eino/schema"
	"github.com/yourusername/goagentpro/internal/store"
	"go.uber.org/zap"
)

// ConversationService wraps the conversation store.
// This layer exists for consistency across services and to host future
// business logic (e.g. conversation-level rate limiting, archival).
type ConversationService struct {
	store  *store.ConversationMemory
	logger *zap.Logger
}

// NewConversationService creates a ConversationService.
func NewConversationService(s *store.ConversationMemory, logger *zap.Logger) *ConversationService {
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

// Create creates a new conversation and returns its ID.
func (s *ConversationService) Create(ctx context.Context, title string) string {
	id := store.NewConversationID()
	s.store.Create(ctx, id, title)
	return id
}

// List returns all conversations, newest first.
func (s *ConversationService) List(ctx context.Context) []ConversationMeta {
	metas := s.store.List(ctx)
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
	return result
}

// LoadMessages returns messages for a conversation.
func (s *ConversationService) LoadMessages(ctx context.Context, id string) ([]*schema.Message, error) {
	return s.store.Load(ctx, id)
}

// SaveMessages appends messages to a conversation.
func (s *ConversationService) SaveMessages(ctx context.Context, id string, msgs []*schema.Message) error {
	return s.store.Save(ctx, id, msgs)
}
