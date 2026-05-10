package model

// CreateConversationRequest is the JSON body for creating a conversation.
type CreateConversationRequest struct {
	Title string `json:"title"`
}

// CreateConversationResponse is the response from creating a conversation.
type CreateConversationResponse struct {
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title"`
	CreatedAt      string `json:"created_at"`
}

// ConversationItem is a single conversation in list responses.
type ConversationItem struct {
	ConversationID string `json:"conversation_id"`
	Title          string `json:"title"`
	MessageCount   int    `json:"message_count"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// ListConversationsResponse is the paginated list response.
type ListConversationsResponse struct {
	Total         int                `json:"total"`
	Conversations []ConversationItem `json:"conversations"`
}

// MessageItem is a single message in conversation history.
type MessageItem struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// GetMessagesResponse holds the messages for a conversation.
type GetMessagesResponse struct {
	ConversationID string        `json:"conversation_id"`
	Total          int           `json:"total"`
	Messages       []MessageItem `json:"messages"`
}
