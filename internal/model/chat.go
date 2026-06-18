package model

// ChatRequest is the JSON body for POST /api/v1/chat.
type ChatRequest struct {
	Question       string `json:"question" binding:"required"`
	Stream         bool   `json:"stream"`
	Agent          bool   `json:"agent"`
	StockMode      bool   `json:"stock_mode"`
	ConversationID string `json:"conversation_id"`
}

// ChatResponse is the standard non-streaming response body.
type ChatResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    ChatResponseData `json:"data"`
}

// ChatResponseData holds the result payload.
type ChatResponseData struct {
	Content string `json:"content"`
	Role    string `json:"role"`
}

// StreamEvent is the SSE event envelope for streaming responses.
type StreamEvent struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
	Tool    string `json:"tool,omitempty"`
	ToolName string `json:"tool_name,omitempty"`
	ToolArgs string `json:"tool_args,omitempty"`
}

const (
	EventToken           = "token"
	EventToolCall        = "tool_call"
	EventToolResult      = "tool_result"
	EventDone            = "done"
	EventError           = "error"
	EventConversationID  = "conversation_id"
)
