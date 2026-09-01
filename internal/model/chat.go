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

// SourceRef 回复引用的数据来源（web 搜索结果 / 知识库文档 / 行情数据源）。
type SourceRef struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"` // 知识库等内部来源可为空
	Kind  string `json:"kind"`          // web / knowledge / stock
}

// StreamEvent is the SSE event envelope for streaming responses.
type StreamEvent struct {
	Type     string      `json:"type"`
	Content  string      `json:"content,omitempty"`
	Tool     string      `json:"tool,omitempty"`
	ToolName string      `json:"tool_name,omitempty"`
	ToolArgs string      `json:"tool_args,omitempty"`
	Sources  []SourceRef `json:"sources,omitempty"`
}

const (
	EventToken          = "token"
	EventToolCall       = "tool_call"
	EventToolResult     = "tool_result"
	EventDone           = "done"
	EventError          = "error"
	EventConversationID = "conversation_id"
	EventWaiting        = "waiting" // 排队等待中，content 含排队信息
	EventPhase          = "phase"   // Agent 思考阶段，content 含当前步骤描述
	EventSources        = "sources" // 回复引用的数据来源，done 前发一次
)
