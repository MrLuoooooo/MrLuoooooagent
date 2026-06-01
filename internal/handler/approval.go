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
// @Summary      列出待审批项
// @Description  获取所有待审批的定时任务操作。
// @Tags         审批
// @Produce      json
// @Success      200 {object} model.APIEnvelope{data=[]model.ApprovalItem}
// @Router       /approvals/pending [get]
func (h *ApprovalHandler) ListPending(c *gin.Context) {
	items := h.store.Pending()
	if items == nil {
		items = []*model.ApprovalItem{}
	}
	c.JSON(http.StatusOK, model.OK(items))
}

// ListAll 列出全部审批项。
// @Summary      列出全部审批项
// @Description  获取所有审批项（包含已处理和待处理的）。
// @Tags         审批
// @Produce      json
// @Success      200 {object} model.APIEnvelope{data=[]model.ApprovalItem}
// @Router       /approvals [get]
func (h *ApprovalHandler) ListAll(c *gin.Context) {
	items := h.store.All()
	if items == nil {
		items = []*model.ApprovalItem{}
	}
	c.JSON(http.StatusOK, model.OK(items))
}

// Decide 接受或拒绝某个审批项。
// @Summary      审批决策
// @Description  接受或拒绝一个待审批的定时任务操作。
// @Tags         审批
// @Accept       json
// @Produce      json
// @Param        approval_id path string true "审批项 ID"
// @Param        request body object{accept=bool} true "决策（accept=true 接受，false 拒绝）"
// @Success      200 {object} model.APIEnvelope
// @Failure      400 {object} model.APIEnvelope
// @Router       /approvals/{approval_id}/decide [post]
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
