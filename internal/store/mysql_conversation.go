package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MySQLConversationStore 基于 MySQL 的会话存储。
// 方法集与 service.ConversationStore 完全一致，隐式实现该接口（LSP）。
type MySQLConversationStore struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewMySQLConversationStore —
func NewMySQLConversationStore(db *gorm.DB, logger *zap.Logger) *MySQLConversationStore {
	return &MySQLConversationStore{db: db, logger: logger}
}

// Create 注册新会话。
func (s *MySQLConversationStore) Create(ctx context.Context, id, title string) error {
	now := time.Now().UTC()
	row := mysqlConv{ID: id, Title: title, CreatedAt: now, UpdatedAt: now}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		s.logger.Error("mysql create conversation", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("mysql create conversation: %w", err)
	}
	return nil
}

// List 列全部会话元数据，最新在前。
func (s *MySQLConversationStore) List(ctx context.Context) ([]ConversationMeta, error) {
	var rows []mysqlConv
	if err := s.db.WithContext(ctx).
		Order("updated_at DESC").
		Limit(1000).
		Find(&rows).Error; err != nil {
		s.logger.Error("mysql list conversations", zap.Error(err))
		return nil, fmt.Errorf("mysql list conversations: %w", err)
	}
	result := make([]ConversationMeta, len(rows))
	for i, r := range rows {
		result[i] = ConversationMeta{
			ID:           r.ID,
			Title:        r.Title,
			MessageCount: r.MessageCount,
			CreatedAt:    r.CreatedAt,
			UpdatedAt:    r.UpdatedAt,
		}
	}
	return result, nil
}

// Load 取会话消息，按写入顺序升序。
func (s *MySQLConversationStore) Load(ctx context.Context, conversationID string) ([]*schema.Message, error) {
	var rows []mysqlMsg
	if err := s.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("msg_order ASC").
		Limit(1000).
		Find(&rows).Error; err != nil {
		s.logger.Error("mysql load messages", zap.String("conv_id", conversationID), zap.Error(err))
		return nil, fmt.Errorf("mysql load messages: %w", err)
	}
	result := make([]*schema.Message, len(rows))
	for i, r := range rows {
		msg := &schema.Message{Role: schema.RoleType(r.Role), Content: r.Content}
		if r.ToolCalls != "" && r.ToolCalls != "[]" {
			var tcs []schema.ToolCall
			if err := json.Unmarshal([]byte(r.ToolCalls), &tcs); err == nil {
				msg.ToolCalls = tcs
			}
		}
		result[i] = msg
	}
	return result, nil
}

// Save 事务：写消息 + 更新会话计数，保证原子性（ES 版不具备的能力）。
func (s *MySQLConversationStore) Save(ctx context.Context, conversationID string, msgs []*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]mysqlMsg, 0, len(msgs))
	base := time.Now().UnixNano()
	for i, msg := range msgs {
		rows = append(rows, mysqlMsg{
			ConversationID: conversationID,
			Role:           string(msg.Role),
			Content:        msg.Content,
			ToolCalls:      marshalToolCalls(msg.ToolCalls),
			CreatedAt:      now,
			MsgOrder:       base + int64(i), // 同批内严格递增，保证顺序
		})
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("mysql insert messages: %w", err)
		}
		res := tx.Model(&mysqlConv{}).Where("id = ?", conversationID).
			Updates(map[string]any{
				"message_count": gorm.Expr("message_count + ?", len(rows)),
				"updated_at":    now,
			})
		if res.Error != nil {
			return fmt.Errorf("mysql update conversation count: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			// 会话不存在：回滚消息写入，避免孤儿消息与计数失真（ES 版无此保障）。
			return fmt.Errorf("mysql conversation %s not found", conversationID)
		}
		return nil
	})
	if err != nil {
		s.logger.Error("mysql save messages", zap.String("conv_id", conversationID), zap.Error(err))
		return err
	}
	return nil
}

// Delete 事务：删消息 + 删会话。
func (s *MySQLConversationStore) Delete(ctx context.Context, conversationID string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", conversationID).
			Delete(&mysqlMsg{}).Error; err != nil {
			return fmt.Errorf("mysql delete messages: %w", err)
		}
		if err := tx.Where("id = ?", conversationID).
			Delete(&mysqlConv{}).Error; err != nil {
			return fmt.Errorf("mysql delete conversation: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.Error("mysql delete conversation", zap.String("conv_id", conversationID), zap.Error(err))
		return err
	}
	return nil
}

// UpdateTitle 更新标题与更新时间。
func (s *MySQLConversationStore) UpdateTitle(ctx context.Context, id, title string) error {
	if err := s.db.WithContext(ctx).Model(&mysqlConv{}).
		Where("id = ?", id).
		Updates(map[string]any{"title": title, "updated_at": time.Now().UTC()}).Error; err != nil {
		s.logger.Error("mysql update title", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("mysql update conversation title: %w", err)
	}
	return nil
}

// DeleteAll 清空全部会话及消息。
func (s *MySQLConversationStore) DeleteAll(ctx context.Context) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&mysqlMsg{}).Error; err != nil {
			return fmt.Errorf("mysql delete all messages: %w", err)
		}
		if err := tx.Where("1 = 1").Delete(&mysqlConv{}).Error; err != nil {
			return fmt.Errorf("mysql delete all conversations: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.Error("mysql delete all", zap.Error(err))
		return err
	}
	return nil
}

// marshalToolCalls 序列化 ToolCalls 为 JSON 字符串；空则返回 "[]"（合法 JSON，MySQL JSON 列不允许空串）。
func marshalToolCalls(tcs []schema.ToolCall) string {
	if len(tcs) == 0 {
		return "[]"
	}
	b, err := json.Marshal(tcs)
	if err != nil {
		return "[]"
	}
	return string(b)
}
