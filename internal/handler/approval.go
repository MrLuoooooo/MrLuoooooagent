package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// ApprovalHandler 管审批列表和决策。
type ApprovalHandler struct {
	store  *service.ApprovalStore
	logger *zap.Logger
}

// NewApprovalHandler —
func NewApprovalHandler(store *service.ApprovalStore, logger *zap.Logger) *ApprovalHandler {
	return &ApprovalHandler{store: store, logger: logger}
}

// ListPending 列出待审批项。
func (h *ApprovalHandler) ListPending(c *gin.Context) {
	items := h.store.Pending()
	if items == nil {
		items = []*model.ApprovalItem{}
	}
	c.JSON(http.StatusOK, model.OK(items))
}

// ListAll 列出全部审批项。
func (h *ApprovalHandler) ListAll(c *gin.Context) {
	items := h.store.All()
	if items == nil {
		items = []*model.ApprovalItem{}
	}
	c.JSON(http.StatusOK, model.OK(items))
}

// Decide 接受或拒绝某个审批项。
func (h *ApprovalHandler) Decide(c *gin.Context) {
	id := c.Param("approval_id")
	var req struct {
		Accept bool `json:"accept"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	if err := h.store.Decide(id, req.Accept); err != nil {
		h.logger.Warn("decide approval", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	action := "rejected"
	if req.Accept {
		action = "accepted"
	}
	c.JSON(http.StatusOK, model.OK(gin.H{"id": id, "action": action}))
}
