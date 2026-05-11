package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/cloudwego/eino/schema"
	"github.com/yourusername/goagentpro/internal/model"
	"github.com/yourusername/goagentpro/internal/service"
	"go.uber.org/zap"
)

// ChatHandler handles POST /api/v1/chat via the ChatService.
// Persistence is delegated to ChatService — handler only does request I/O.
type ChatHandler struct {
	svc    *service.ChatService
	convSvc *service.ConversationService
	logger *zap.Logger
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
	msg, err := h.svc.Chat(c.Request.Context(), req.Question, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
}

func (h *ChatHandler) handleStream(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	// Save user message immediately, before starting the LLM stream.
	// This ensures the question persists even if the user refreshes mid-stream.
	if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
		h.logger.Error("save user message before stream", zap.String("conv_id", convID), zap.Error(err))
	}

	// SSE headers MUST be flushed before any blocking call (ChatStream may take long)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	stream, err := h.svc.ChatStream(c.Request.Context(), req.Question)
	if err != nil {
		evt, _ := json.Marshal(model.StreamEvent{Type: model.EventError, Content: err.Error()})
		c.Writer.Write([]byte("data: " + string(evt) + "\n\n"))
		c.Writer.Flush()
		return
	}
	defer stream.Close()

	var full string
	c.Stream(func(w io.Writer) bool {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
				h.sendConvIDEvent(w, convID)
				done, _ := json.Marshal(model.StreamEvent{Type: model.EventDone})
				w.Write([]byte("data: " + string(done) + "\n\n"))
				return false
			}
			return false
		}
		full += chunk.Content
		evt, _ := json.Marshal(model.StreamEvent{
			Type:    model.EventToken,
			Content: chunk.Content,
		})
		w.Write([]byte("data: " + string(evt) + "\n\n"))
		return true
	})

	// Persist assistant response after stream ends
	// (user message was already saved by SaveUserMessage above)
	if full != "" {
		if err := h.svc.SaveAssistantMessage(convID, full, nil); err != nil {
			h.logger.Error("save stream assistant reply", zap.String("conv_id", convID), zap.Error(err))
		}
	}
}

func (h *ChatHandler) handleAgent(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	if req.Stream {
		// Save user message immediately, before starting the LLM stream
		if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
			h.logger.Error("save user message before agent stream", zap.String("conv_id", convID), zap.Error(err))
		}

		// SSE headers MUST be flushed before any blocking call (AgentStream may take long)
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Status(http.StatusOK)
		c.Writer.Flush()

		userMsg := &schema.Message{Role: schema.User, Content: req.Question}
		stream, err := h.svc.AgentStream(c.Request.Context(), userMsg)
		if err != nil {
			evt, _ := json.Marshal(model.StreamEvent{Type: model.EventError, Content: err.Error()})
			c.Writer.Write([]byte("data: " + string(evt) + "\n\n"))
			c.Writer.Flush()
			return
		}
		defer stream.Close()

		var full string
		var toolCalls []schema.ToolCall
		c.Stream(func(w io.Writer) bool {
			chunk, recvErr := stream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					h.sendConvIDEvent(w, convID)
					done, _ := json.Marshal(model.StreamEvent{Type: model.EventDone})
					w.Write([]byte("data: " + string(done) + "\n\n"))
					return false
				}
				return false
			}
			full += chunk.Content
			if len(chunk.ToolCalls) > 0 {
				toolCalls = chunk.ToolCalls
			}
			evt, _ := json.Marshal(model.StreamEvent{
				Type:    model.EventToken,
				Content: chunk.Content,
			})
			w.Write([]byte("data: " + string(evt) + "\n\n"))
			return true
		})
		if full != "" {
			if err := h.svc.SaveAssistantMessage(convID, full, toolCalls); err != nil {
				h.logger.Error("save agent assistant reply", zap.String("conv_id", convID), zap.Error(err))
			}
		}
		return
	}

	// Non-streaming agent
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

func (h *ChatHandler) sendConvIDEvent(w io.Writer, convID string) {
	evt, _ := json.Marshal(model.StreamEvent{
		Type:    model.EventConversationID,
		Content: convID,
	})
	w.Write([]byte("data: " + string(evt) + "\n\n"))
}

func prependHistory(q string, history []*schema.Message) string {
	var b strings.Builder
	for _, m := range history {
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
		b.WriteString("\n")
	}
	return "History:\n" + b.String() + "\nUser: " + q
}
