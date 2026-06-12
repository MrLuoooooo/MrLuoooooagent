package handler

import (
	"net/http"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// FeedbackHandler 处理用户反馈请求。
type FeedbackHandler struct {
	svc    *service.FeedbackService
	logger *zap.Logger
}

// NewFeedbackHandler —
func NewFeedbackHandler(svc *service.FeedbackService, logger *zap.Logger) *FeedbackHandler {
	return &FeedbackHandler{svc: svc, logger: logger}
}

// SubmitFeedback POST /api/v1/feedback
func (h *FeedbackHandler) SubmitFeedback(c *gin.Context) {
	var req model.FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid request: " + err.Error()})
		return
	}

	item, err := h.svc.RecordFeedback(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("record feedback failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": item})
}
