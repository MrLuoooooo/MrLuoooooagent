package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// SkillHandler 管技能的增删查。
type SkillHandler struct {
	store  *service.SkillStore
	logger *zap.Logger
}

func NewSkillHandler(store *service.SkillStore, logger *zap.Logger) *SkillHandler {
	return &SkillHandler{store: store, logger: logger}
}

// List 列出所有技能。
// @Summary      列出技能
// @Description  获取所有用户自定义技能列表。技能会在 Agent 运行时注入到系统提示词中。
// @Tags         技能
// @Produce      json
// @Success      200 {object} model.APIEnvelope
// @Router       /skills [get]
func (h *SkillHandler) List(c *gin.Context) {
	skills := h.store.All()
	if skills == nil {
		skills = make([]service.SkillEntry, 0)
	}
	c.JSON(http.StatusOK, model.OK(skills))
}

// Upsert adds or updates a skill.
// @Summary      添加/更新技能
// @Description  添加或更新一个用户自定义技能。技能会在 Agent 执行时自动注入提示词。
// @Tags         技能
// @Accept       json
// @Produce      json
// @Param        request body service.SkillEntry true "技能配置（name + prompt）"
// @Success      200 {object} model.APIEnvelope
// @Failure      400 {object} model.APIEnvelope
// @Failure      500 {object} model.APIEnvelope
// @Router       /skills [post]
func (h *SkillHandler) Upsert(c *gin.Context) {
	var entry service.SkillEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	if entry.Name == "" || entry.Prompt == "" {
		c.JSON(http.StatusBadRequest, model.Err(400, "name and prompt are required"))
		return
	}
	if err := h.store.AddOrUpdate(entry); err != nil {
		h.logger.Warn("upsert skill", zap.String("name", entry.Name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(gin.H{"name": entry.Name, "message": "skill saved"}))
}

// Remove deletes a skill by name.
// @Summary      删除技能
// @Description  按名称删除一个用户自定义技能。
// @Tags         技能
// @Produce      json
// @Param        name path string true "技能名称"
// @Success      200 {object} model.APIEnvelope
// @Failure      400 {object} model.APIEnvelope
// @Router       /skills/{name} [delete]
func (h *SkillHandler) Remove(c *gin.Context) {
	name := c.Param("name")
	if err := h.store.Remove(name); err != nil {
		h.logger.Warn("remove skill", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(gin.H{"name": name, "message": "skill removed"}))
}
