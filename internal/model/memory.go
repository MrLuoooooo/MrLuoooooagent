package model

import "time"

// MemoryType 区分记忆的语义类别。
type MemoryType string

const (
	MemoryTypeFact       MemoryType = "fact"       // 用户陈述的事实
	MemoryTypePreference MemoryType = "preference" // 用户偏好
	MemoryTypeDecision   MemoryType = "decision"   // 过往决策
	MemoryTypeKnowledge  MemoryType = "knowledge"  // 系统学到的知识
)

// MemoryStatus 标记记忆的生命周期。
type MemoryStatus string

const (
	MemoryActive     MemoryStatus = "active"
	MemorySuperseded MemoryStatus = "superseded"
	MemoryDeprecated MemoryStatus = "deprecated"
)

// UserMemory 是 API 层面的记忆模型，不包含存储实现细节。
type UserMemory struct {
	ID        string       `json:"id"`
	UserID    string       `json:"user_id"`
	Type      MemoryType   `json:"type"`
	Content   string       `json:"content"`
	Keywords  []string     `json:"keywords"`
	Source    string       `json:"source"`
	Status    MemoryStatus `json:"status"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	Version   int          `json:"version"`
}

// MemorySearchRequest 检索请求。
type MemorySearchRequest struct {
	Query string `json:"query" form:"query" binding:"required"`
	TopK  int    `json:"top_k" form:"top_k"`
}

// MemorySearchResponse 检索结果。
type MemorySearchResponse struct {
	Memories []UserMemory `json:"memories"`
	Total    int          `json:"total"`
}
