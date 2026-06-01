package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// ConversationHandler 管会话的增删查。
type ConversationHandler struct {
	svc    *service.ConversationService
	logger *zap.Logger
}

// NewConversationHandler —
func NewConversationHandler(svc *service.ConversationService, logger *zap.Logger) *ConversationHandler {
	return &ConversationHandler{svc: svc, logger: logger}
}

// CreateConversation 建新会话。
func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	var req model.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Title = "新会话"
	}
	id, err := h.svc.Create(c.Request.Context(), req.Title)
	if err != nil {
		h.logger.Error("create conversation", zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(model.CreateConversationResponse{
		ConversationID: id,
		Title:          req.Title,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
	}))
}

// ListConversations 列全部会话。
func (h *ConversationHandler) ListConversations(c *gin.Context) {
	convs, err := h.svc.List(c.Request.Context())
	if err != nil {
		h.logger.Error("list conversations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
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

// GetMessages 取某个会话的历史消息。
func (h *ConversationHandler) GetMessages(c *gin.Context) {
	convID := c.Param("conversation_id")
	msgs, err := h.svc.LoadMessages(c.Request.Context(), convID)
	if err != nil {
		h.logger.Error("get messages", zap.String("conv_id", convID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	items := make([]model.MessageItem, len(msgs))
	for i, m := range msgs {
		mi := model.MessageItem{Role: string(m.Role), Content: m.Content}
		if len(m.ToolCalls) > 0 {
			mi.ToolCalls = make([]model.ToolCall, len(m.ToolCalls))
			for j, tc := range m.ToolCalls {
				mi.ToolCalls[j] = model.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: model.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		items[i] = mi
	}
	c.JSON(http.StatusOK, model.OK(model.GetMessagesResponse{
		ConversationID: convID, Total: len(items), Messages: items,
	}))
}

// DeleteConversation 删一个会话。
func (h *ConversationHandler) DeleteConversation(c *gin.Context) {
	convID := c.Param("conversation_id")
	if err := h.svc.Delete(c.Request.Context(), convID); err != nil {
		h.logger.Error("delete conversation", zap.String("conv_id", convID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(model.DeleteConversationResponse{
		ConversationID: convID,
	}))
}

// DeleteAllConversations 清空全部会话及消息。
func (h *ConversationHandler) DeleteAllConversations(c *gin.Context) {
	if err := h.svc.DeleteAll(c.Request.Context()); err != nil {
		h.logger.Error("delete all conversations", zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(map[string]string{"status": "ok"}))
}
