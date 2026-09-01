package store

import (
	"time"

	"gorm.io/gorm"
)

// MySQL 业务主库表模型。
// 与 ES 版（ESConversationStore / ESFeedbackStore）平级，
// 通过实现 service 层接口参与注入，业务代码不感知后端差异（LSP）。

// mysqlConv 会话元数据表模型。
type mysqlConv struct {
	ID           string    `gorm:"column:id;primaryKey;size:64"`
	Title        string    `gorm:"column:title;size:512;not null;default:''"`
	MessageCount int       `gorm:"column:message_count;not null;default:0"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (mysqlConv) TableName() string { return "conversations" }

// mysqlMsg 单条消息表模型。
// msg_order 用时间戳保证写入顺序；order 是 MySQL 保留字，故命名 msg_order。
type mysqlMsg struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	ConversationID string    `gorm:"column:conversation_id;size:64;index:idx_conv_order,priority:1"`
	Role           string    `gorm:"column:role;size:32;not null"`
	Content        string    `gorm:"column:content;type:mediumtext"`
	ToolCalls      string    `gorm:"column:tool_calls;type:json;default:null"` // schema.ToolCall 序列化 JSON
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
	MsgOrder       int64     `gorm:"column:msg_order;index:idx_conv_order,priority:2"`
}

func (mysqlMsg) TableName() string { return "messages" }

// mysqlFeedback 反馈表模型。
type mysqlFeedback struct {
	ID             string    `gorm:"column:id;primaryKey;size:64"`
	ConversationID string    `gorm:"column:conversation_id;size:64;index"`
	MessageIndex   int       `gorm:"column:message_index;not null"`
	Type           string    `gorm:"column:type;size:32;not null"`
	Rating         int       `gorm:"column:rating;not null;default:0"`
	CorrectAnswer  string    `gorm:"column:correct_answer;type:text"`
	Comment        string    `gorm:"column:comment;type:text"`
	SourceQuery    string    `gorm:"column:source_query;type:text"`
	SourceAnswer   string    `gorm:"column:source_answer;type:text"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
}

func (mysqlFeedback) TableName() string { return "feedbacks" }

// AutoMigrateMySQL 建表/迁移 + FULLTEXT(ngram) 索引，由注入层启动时调用。
func AutoMigrateMySQL(db *gorm.DB) error {
	if err := db.AutoMigrate(&mysqlConv{}, &mysqlMsg{}, &mysqlFeedback{}, &DocumentChunk{}, &mysqlDocMeta{}, &mysqlMemory{}); err != nil {
		return err
	}
	return ensureFulltextIndexes(db)
}
