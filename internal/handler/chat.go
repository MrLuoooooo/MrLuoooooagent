package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/yourusername/goagentpro/internal/model"
	"github.com/yourusername/goagentpro/internal/service"
	"go.uber.org/zap"
)

// ChatHandler handles POST /api/v1/chat via the ChatService.
type ChatHandler struct {
	svc     *service.ChatService
	convSvc *service.ConversationService
	logger  *zap.Logger
}

// NewChatHandler creates a ChatHandler.
func NewChatHandler(svc *service.ChatService, convSvc *service.ConversationService, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{svc: svc, convSvc: convSvc, logger: logger}
}

// Chat handles POST /api/v1/chat.
func (h *ChatHandler) Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}

	ctx := c.Request.Context()

	// Auto-create conversation if no ID provided
	convID := req.ConversationID
	if convID == "" {
		id, err := h.convSvc.Create(ctx, "新会话")
		if err != nil {
			h.logger.Error("create conversation", zap.Error(err))
			c.JSON(http.StatusInternalServerError, model.Err(500, "创建会话失败"))
			return
		}
		convID = id
	}

	// Load history for existing conversations
	var history []*schema.Message
	if req.ConversationID != "" {
		var err error
		history, err = h.convSvc.LoadMessages(ctx, convID)
		if err != nil {
			h.logger.Error("load history", zap.String("conv_id", convID), zap.Error(err))
		}
	}

	// Save original question BEFORE prependHistory modifies it
	originalQuestion := req.Question

	// Inject history into question for LLM context (does NOT affect saved messages)
	if len(history) > 0 {
		req.Question = prependHistory(req.Question, history)
	}

	if req.Agent {
		h.handleAgent(c, req, convID, originalQuestion)
	} else if req.Stream {
		h.handleStream(c, req, convID, originalQuestion)
	} else {
		h.handleInvoke(c, req, convID, originalQuestion)
	}
}

func (h *ChatHandler) handleInvoke(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	// Save user message with original question (before prependHistory).
	if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
		h.logger.Error("save user message before invoke", zap.String("conv_id", convID), zap.Error(err))
	}

	msg, err := h.svc.Chat(c.Request.Context(), req.Question, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
}

// ── Streaming helpers ───────────────────────────────────────────────

func (h *ChatHandler) handleStream(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
		h.logger.Error("save user message before stream", zap.String("conv_id", convID), zap.Error(err))
	}

	h.setupSSE(c)

	stream, err := h.svc.ChatStream(c.Request.Context(), req.Question)
	if err != nil {
		h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventError, Content: err.Error()})
		return
	}
	defer stream.Close()

	var full string
	h.streamSSE(c, convID, stream, func(chunk *schema.Message) *model.StreamEvent {
		full += chunk.Content
		return &model.StreamEvent{Type: model.EventToken, Content: chunk.Content}
	})

	if full != "" {
		if err := h.svc.SaveAssistantMessage(convID, full, nil); err != nil {
			h.logger.Error("save stream assistant reply", zap.String("conv_id", convID), zap.Error(err))
		}
	}
}

func (h *ChatHandler) handleAgent(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	if req.Stream {
		if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
			h.logger.Error("save user message before agent stream", zap.String("conv_id", convID), zap.Error(err))
		}

		h.setupSSE(c)

		userMsg := &schema.Message{Role: schema.User, Content: req.Question}
		stream, err := h.svc.AgentStream(c.Request.Context(), userMsg)
		if err != nil {
			h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventError, Content: err.Error()})
			return
		}
		defer stream.Close()

		var full string
		var toolCalls []schema.ToolCall
		seenTools := make(map[string]bool) // dedup tool_call events

		h.streamSSE(c, convID, stream, func(chunk *schema.Message) *model.StreamEvent {
			// Emit tool_call events for new tool calls.
			for _, tc := range chunk.ToolCalls {
				if !seenTools[tc.ID] {
					seenTools[tc.ID] = true
					h.writeSSEEvent(c.Writer, model.StreamEvent{
						Type: model.EventToolCall,
						Tool:  fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments),
						ToolName: tc.Function.Name,
						ToolArgs: tc.Function.Arguments,
					})
				}
			}

			// Emit tool_result when chunk is a tool response (Role=Tool).
			if chunk.Role == schema.Tool {
				return &model.StreamEvent{
					Type:    model.EventToolResult,
					Content: chunk.Content,
				}
			}

			full += chunk.Content
			if len(chunk.ToolCalls) > 0 {
				toolCalls = chunk.ToolCalls
			}
			return &model.StreamEvent{Type: model.EventToken, Content: chunk.Content}
		})

		if full != "" {
			if err := h.svc.SaveAssistantMessage(convID, full, toolCalls); err != nil {
				h.logger.Error("save agent assistant reply", zap.String("conv_id", convID), zap.Error(err))
			}
		}
		return
	}

	// Non-streaming agent — save user message with original question first.
	if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
		h.logger.Error("save user message before agent", zap.String("conv_id", convID), zap.Error(err))
	}
	userMsg := &schema.Message{Role: schema.User, Content: req.Question}
	msg, err := h.svc.Agent(c.Request.Context(), userMsg, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
}

// setupSSE writes SSE headers and flushes them.
func (h *ChatHandler) setupSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	c.Writer.Flush()
}

// writeSSEEvent marshals and writes a single SSE event.
func (h *ChatHandler) writeSSEEvent(w io.Writer, evt model.StreamEvent) {
	data, _ := json.Marshal(evt)
	w.Write([]byte("data: " + string(data) + "\n\n"))
}

// emitFn produces an SSE event from a stream chunk, or nil to skip.
type emitFn func(chunk *schema.Message) *model.StreamEvent

// streamSSE is the common SSE read loop used by both RAG and Agent streaming.
func (h *ChatHandler) streamSSE(c *gin.Context, convID string, stream *schema.StreamReader[*schema.Message], emit emitFn) {
	c.Stream(func(w io.Writer) bool {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				h.sendConvIDEvent(w, convID)
				h.writeSSEEvent(w, model.StreamEvent{Type: model.EventDone})
				return false
			}
			h.logger.Warn("stream recv error, sending error to client", zap.Error(err))
				h.writeSSEEvent(w, model.StreamEvent{Type: model.EventError, Content: err.Error()})
			return false
		}

		if evt := emit(chunk); evt != nil {
			h.writeSSEEvent(w, *evt)
		}
		return true
	})
}

func (h *ChatHandler) sendConvIDEvent(w io.Writer, convID string) {
	h.writeSSEEvent(w, model.StreamEvent{
		Type:    model.EventConversationID,
		Content: convID,
	})
}

// ── History formatting ────────────────────────────────────────────

func prependHistory(q string, history []*schema.Message) string {
	const maxHistory = 20        // max messages to keep
	const maxContentLen = 2000    // max chars per message

	// Take the last N messages to avoid context overflow.
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	truncate := func(s string) string {
		if len(s) > maxContentLen {
			return s[:maxContentLen] + "...[截断]"
		}
		return s
	}

	var b strings.Builder
	b.WriteString("以下是之前的对话历史:\n")
	for _, m := range history {
		switch m.Role {
		case schema.Tool:
			b.WriteString("tool_result: ")
			b.WriteString(truncate(m.Content))
			b.WriteString("\n")
		case schema.Assistant:
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					b.WriteString("assistant: [调用工具 ")
					b.WriteString(tc.Function.Name)
					b.WriteString("，参数: ")
					b.WriteString(truncate(tc.Function.Arguments))
					b.WriteString("]\n")
				}
			}
			if m.Content != "" {
				b.WriteString("assistant: ")
				b.WriteString(truncate(m.Content))
				b.WriteString("\n")
			}
		default:
			b.WriteString(string(m.Role))
			b.WriteString(": ")
			b.WriteString(truncate(m.Content))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n用户当前问题: ")
	b.WriteString(q)
	return b.String()
}
