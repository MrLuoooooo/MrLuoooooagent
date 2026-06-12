package model

import "time"

// FeedbackType 用户反馈类型。
type FeedbackType string

const (
	FeedbackThumbsUp   FeedbackType = "thumbs_up"
	FeedbackThumbsDown FeedbackType = "thumbs_down"
	FeedbackRating     FeedbackType = "rating"
	FeedbackCorrection FeedbackType = "correction" // 用户手动纠正
)

// FeedbackItem 一条用户反馈。
type FeedbackItem struct {
	ID             string       `json:"id"`
	ConversationID string       `json:"conversation_id"`
	MessageIndex   int          `json:"message_index"` // 对话中第几条助手回复
	Type           FeedbackType `json:"type"`
	Rating         int          `json:"rating,omitempty"`      // 1-5 评分
	CorrectAnswer  string       `json:"correct_answer,omitempty"` // 用户纠正的正确答案
	Comment        string       `json:"comment,omitempty"`
	SourceQuery    string       `json:"source_query"`   // 当时的用户问题
	SourceAnswer   string       `json:"source_answer"`  // 当时的助手回答
	CreatedAt      time.Time    `json:"created_at"`
}

// FeedbackRequest HTTP 请求。
type FeedbackRequest struct {
	ConversationID string       `json:"conversation_id" binding:"required"`
	MessageIndex   int          `json:"message_index" binding:"required"`
	Type           FeedbackType `json:"type" binding:"required"`
	Rating         int          `json:"rating"`
	CorrectAnswer  string       `json:"correct_answer"`
	Comment        string       `json:"comment"`
	SourceQuery    string       `json:"source_query"`
	SourceAnswer   string       `json:"source_answer"`
}
