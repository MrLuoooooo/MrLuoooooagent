package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MySQLFeedbackStore 基于 MySQL 的反馈存储。
// 方法集与 service.FeedbackStore 一致，隐式实现该接口（LSP）。
type MySQLFeedbackStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewMySQLFeedbackStore —
func NewMySQLFeedbackStore(db *gorm.DB, logger *zap.Logger) *MySQLFeedbackStore {
	return &MySQLFeedbackStore{db: db, logger: logger}
}

// Save 存一条反馈。
func (s *MySQLFeedbackStore) Save(ctx context.Context, item *model.FeedbackItem) error {
	row := mysqlFeedback{
		ID:             item.ID,
		ConversationID: item.ConversationID,
		MessageIndex:   item.MessageIndex,
		Type:           string(item.Type),
		Rating:         item.Rating,
		CorrectAnswer:  item.CorrectAnswer,
		Comment:        item.Comment,
		SourceQuery:    item.SourceQuery,
		SourceAnswer:   item.SourceAnswer,
		CreatedAt:      item.CreatedAt.UTC(),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		s.logger.Error("mysql save feedback", zap.String("id", item.ID), zap.Error(err))
		return fmt.Errorf("mysql save feedback: %w", err)
	}
	return nil
}

// List 按 conversation_id 查询，最新在前。
func (s *MySQLFeedbackStore) List(ctx context.Context, conversationID string) ([]*model.FeedbackItem, error) {
	var rows []mysqlFeedback
	if err := s.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Limit(100).
		Find(&rows).Error; err != nil {
		s.logger.Error("mysql list feedback", zap.String("conv_id", conversationID), zap.Error(err))
		return nil, fmt.Errorf("mysql list feedback: %w", err)
	}
	return toFeedbackItems(rows), nil
}

// ListRecent 最近 N 条（全量）。
func (s *MySQLFeedbackStore) ListRecent(ctx context.Context, limit int) ([]*model.FeedbackItem, error) {
	var rows []mysqlFeedback
	if err := s.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		s.logger.Error("mysql list recent feedback", zap.Int("limit", limit), zap.Error(err))
		return nil, fmt.Errorf("mysql list recent feedback: %w", err)
	}
	return toFeedbackItems(rows), nil
}

func toFeedbackItems(rows []mysqlFeedback) []*model.FeedbackItem {
	result := make([]*model.FeedbackItem, len(rows))
	for i, r := range rows {
		result[i] = &model.FeedbackItem{
			ID:             r.ID,
			ConversationID: r.ConversationID,
			MessageIndex:   r.MessageIndex,
			Type:           model.FeedbackType(r.Type),
			Rating:         r.Rating,
			CorrectAnswer:  r.CorrectAnswer,
			Comment:        r.Comment,
			SourceQuery:    r.SourceQuery,
			SourceAnswer:   r.SourceAnswer,
			CreatedAt:      r.CreatedAt,
		}
	}
	return result
}

// MemFeedbackStore 内存反馈存储，MySQL/ES 均不可用时的降级方案（防 nil panic）。
type MemFeedbackStore struct {
	mu   sync.RWMutex
	rows []*model.FeedbackItem
}

// NewMemFeedbackStore —
func NewMemFeedbackStore() *MemFeedbackStore {
	return &MemFeedbackStore{}
}

func (s *MemFeedbackStore) Save(ctx context.Context, item *model.FeedbackItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, item)
	return nil
}

func (s *MemFeedbackStore) List(ctx context.Context, conversationID string) ([]*model.FeedbackItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*model.FeedbackItem
	for _, it := range s.rows {
		if it.ConversationID == conversationID {
			out = append(out, it)
		}
	}
	return out, nil
}

func (s *MemFeedbackStore) ListRecent(ctx context.Context, limit int) ([]*model.FeedbackItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := len(s.rows)
	if limit > 0 && n > limit {
		n = limit
	}
	out := make([]*model.FeedbackItem, n)
	for i := 0; i < n; i++ {
		out[i] = s.rows[len(s.rows)-1-i] // 始终从末尾取，最近在前
	}
	return out, nil
}
