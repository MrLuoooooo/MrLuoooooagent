// Package memoryimpl 提供对话记忆存储能力。
//
// 注意：Eino v0.8.13 尚未定义 components/memory.Memory 标准接口。
// 当前实现以具体类型 *ConversationMemory 提供，通过 fx 注入到编排层。
// 若后续 Eino 版本提供官方 Memory 接口，需重构为：
//   func NewConversationMemory(...) memory.Memory { ... }
//
// 相关讨论见：https://github.com/cloudwego/eino/issues (Memory 组件需求)
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ConversationMemory stores and retrieves message history.
// It does NOT define a custom interface — it is a concrete service used
// directly by the handlers to persist conversation data.
type ConversationMemory struct {
	mu            sync.RWMutex
	conversations  map[string][]*schema.Message
	meta           map[string]ConversationMeta
	maxHistory     int

	// Maintain insertion order for list queries.
	order []string
}

// ConversationMeta holds basic conversation metadata.
type ConversationMeta struct {
	ID        string    `json:"conversation_id"`
	Title     string    `json:"title"`
	MessageCount int    `json:"message_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewConversationMemory creates an in-memory message store.
// maxHistory caps the number of messages kept per conversation (oldest
// messages are trimmed first).
func NewConversationMemory(maxHistory int) *ConversationMemory {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &ConversationMemory{
		conversations: make(map[string][]*schema.Message),
		meta:          make(map[string]ConversationMeta),
		order:         make([]string, 0),
		maxHistory:   maxHistory,
	}
}

// Save appends messages to a conversation's history and enforces the cap.
func (m *ConversationMemory) Save(ctx context.Context, conversationID string, msgs []*schema.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	history := m.conversations[conversationID]
	history = append(history, msgs...)

	if len(history) > m.maxHistory {
		history = history[len(history)-m.maxHistory:]
	}

	m.conversations[conversationID] = history

	// Update metadata.
	if meta, ok := m.meta[conversationID]; ok {
		meta.MessageCount = len(history)
		meta.UpdatedAt = time.Now()
		m.meta[conversationID] = meta
	}

	return nil
}

// Create registers a new conversation with metadata.
func (m *ConversationMemory) Create(ctx context.Context, id string, title string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.conversations[id]; exists {
		return
	}
	m.conversations[id] = make([]*schema.Message, 0)
	m.meta[id] = ConversationMeta{
		ID:        id,
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.order = append(m.order, id)
}

// List returns metadata for all conversations, newest first.
func (m *ConversationMemory) List(ctx context.Context) []ConversationMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ConversationMeta, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		id := m.order[i]
		if meta, ok := m.meta[id]; ok {
			meta.MessageCount = len(m.conversations[id])
			result = append(result, meta)
		}
	}
	return result
}

// Load returns all stored messages for a conversation.
func (m *ConversationMemory) Load(ctx context.Context, conversationID string) ([]*schema.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := m.conversations[conversationID]
	if history == nil {
		return nil, nil
	}

	// Return a copy to avoid data races.
	result := make([]*schema.Message, len(history))
	for i, msg := range history {
		result[i] = copyMessage(msg)
	}
	return result, nil
}

func copyMessage(msg *schema.Message) *schema.Message {
	if msg == nil {
		return nil
	}
	b, _ := json.Marshal(msg)
	var cp schema.Message
	_ = json.Unmarshal(b, &cp)
	return &cp
}

// Ensure conversation IDs are unique-ish at runtime.
var convCounter int64

// NewConversationID returns a unique conversation ID.
func NewConversationID() string {
	convCounter++
	return fmt.Sprintf("conv_%d_%d", time.Now().UnixNano(), convCounter)
}
