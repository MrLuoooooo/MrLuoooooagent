package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/goagentpro/internal/model"
	"github.com/yourusername/goagentpro/internal/service"
)

// ConversationHandler handles conversation CRUD via ConversationService.
type ConversationHandler struct {
	svc *service.ConversationService
}

// NewConversationHandler creates a ConversationHandler.
func NewConversationHandler(svc *service.ConversationService) *ConversationHandler {
	return &ConversationHandler{svc: svc}
}

// CreateConversation handles POST /api/v1/conversations.
func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	var req model.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Title = "New Conversation"
	}
	id := h.svc.Create(c.Request.Context(), req.Title)
	c.JSON(http.StatusOK, model.OK(model.CreateConversationResponse{
		ConversationID: id,
		Title:          req.Title,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}))
}

// ListConversations handles GET /api/v1/conversations.
func (h *ConversationHandler) ListConversations(c *gin.Context) {
	convs := h.svc.List(c.Request.Context())
	items := make([]model.ConversationItem, len(convs))
	for i, m := range convs {
		items[i] = model.ConversationItem{
			ConversationID: m.ID,
			Title:          m.Title,
			MessageCount:   m.MessageCount,
			CreatedAt:      m.CreatedAt,
			UpdatedAt:      m.UpdatedAt,
		}
	}
	c.JSON(http.StatusOK, model.OK(model.ListConversationsResponse{
		Total: len(items), Conversations: items,
	}))
}

// GetMessages handles GET /api/v1/conversations/:conversation_id/messages.
func (h *ConversationHandler) GetMessages(c *gin.Context) {
	convID := c.Param("conversation_id")
	msgs, err := h.svc.LoadMessages(c.Request.Context(), convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	items := make([]model.MessageItem, len(msgs))
	for i, m := range msgs {
		items[i] = model.MessageItem{Role: string(m.Role), Content: m.Content}
	}
	c.JSON(http.StatusOK, model.OK(model.GetMessagesResponse{
		ConversationID: convID, Total: len(items), Messages: items,
	}))
}
