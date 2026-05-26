package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// ApprovalHandler handles approval list and decision endpoints.
type ApprovalHandler struct {
	store  *service.ApprovalStore
	logger *zap.Logger
}

// NewApprovalHandler creates an ApprovalHandler.
func NewApprovalHandler(store *service.ApprovalStore, logger *zap.Logger) *ApprovalHandler {
	return &ApprovalHandler{store: store, logger: logger}
}

// ListPending returns all pending approvals.
func (h *ApprovalHandler) ListPending(c *gin.Context) {
	items := h.store.Pending()
	if items == nil {
		items = []*model.ApprovalItem{}
	}
	c.JSON(http.StatusOK, model.OK(items))
}

// ListAll returns all approvals.
func (h *ApprovalHandler) ListAll(c *gin.Context) {
	items := h.store.All()
	if items == nil {
		items = []*model.ApprovalItem{}
	}
	c.JSON(http.StatusOK, model.OK(items))
}

// Decide handles accept/reject on a pending approval.
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
