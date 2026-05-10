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
	"github.com/yourusername/goagentpro/internal/store"
	"go.uber.org/zap"
)

// ChatHandler handles POST /api/v1/chat via the ChatService.
type ChatHandler struct {
	svc    *service.ChatService
	memory *store.ConversationMemory
	logger *zap.Logger
}

// NewChatHandler creates a ChatHandler.
func NewChatHandler(svc *service.ChatService, mem *store.ConversationMemory, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{svc: svc, memory: mem, logger: logger}
}

// Chat handles POST /api/v1/chat.
func (h *ChatHandler) Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}

	if req.ConversationID != "" {
		history, err := h.memory.Load(c.Request.Context(), req.ConversationID)
		if err == nil && len(history) > 0 {
			req.Question = prependHistory(req.Question, history)
		}
	}

	if req.Agent {
		h.handleAgent(c, req)
	} else if req.Stream {
		h.handleStream(c, req)
	} else {
		h.handleInvoke(c, req)
	}

	if req.ConversationID != "" {
		h.memory.Save(c.Request.Context(), req.ConversationID, []*schema.Message{
			{Role: schema.User, Content: req.Question},
			{Role: schema.Assistant, Content: extractContent(c)},
		})
	}
}

func (h *ChatHandler) handleInvoke(c *gin.Context, req model.ChatRequest) {
	msg, err := h.svc.Chat(c.Request.Context(), req.Question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	saveContent(c, msg.Content)
	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
}

func (h *ChatHandler) handleStream(c *gin.Context, req model.ChatRequest) {
	stream, err := h.svc.ChatStream(c.Request.Context(), req.Question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	defer stream.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	var full string
	c.Stream(func(w io.Writer) bool {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if recvErr == io.EOF {
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
	saveContent(c, full)
}

func (h *ChatHandler) handleAgent(c *gin.Context, req model.ChatRequest) {
	userMsg := &schema.Message{Role: schema.User, Content: req.Question}
	if req.Stream {
		stream, err := h.svc.AgentStream(c.Request.Context(), userMsg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
			return
		}
		defer stream.Close()
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		var full string
		c.Stream(func(w io.Writer) bool {
			chunk, recvErr := stream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
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
		saveContent(c, full)
		return
	}
	msg, err := h.svc.Agent(c.Request.Context(), userMsg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	saveContent(c, msg.Content)
	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
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

var ctxKeyContent = struct{}{}

func saveContent(c *gin.Context, content string) {
	c.Set("_content", content)
}

func extractContent(c *gin.Context) string {
	if v, ok := c.Get("_content"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
