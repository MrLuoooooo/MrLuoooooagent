package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// ModelSwitcher 模型切换的抽象接口。
type ModelSwitcher interface {
	Switch(modelName string) error
	CurrentName() string
}

// ModelHandler 管模型列表、切换和自定义模型 CRUD。
type ModelHandler struct {
	cfg     *config.Config
	manager ModelSwitcher
	store   *service.ModelStore
	logger  *zap.Logger
}

func NewModelHandler(cfg *config.Config, manager ModelSwitcher, store *service.ModelStore, logger *zap.Logger) *ModelHandler {
	return &ModelHandler{cfg: cfg, manager: manager, store: store, logger: logger}
}

// modelItem is the JSON shape returned by ListAvailable.
type modelItem struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	IsLocal  bool   `json:"is_local"`
	IsCustom bool   `json:"is_custom"`
	Active   bool   `json:"active"`
}

// ListAvailable 列出配置里和用户自定义的全部模型。
func (h *ModelHandler) ListAvailable(c *gin.Context) {
	current := h.manager.CurrentName()
	seen := make(map[string]bool)
	var items []modelItem

	for _, m := range h.cfg.ModelProvider.ModelList {
		items = append(items, h.toItem(m, false, current))
		seen[m.Name] = true
	}
	for _, m := range h.store.All() {
		if seen[m.Name] {
			continue
		}
		items = append(items, h.toItem(m, true, current))
	}
	c.JSON(http.StatusOK, model.OK(items))
}

func (h *ModelHandler) toItem(m config.ModelEntry, isCustom bool, current string) modelItem {
	isLocal := m.Provider == "ollama"
	name := m.Name
	if isLocal {
		name = name + " (本地)"
	}
	return modelItem{
		Name:     name,
		Provider: m.Provider,
		IsLocal:  isLocal,
		IsCustom: isCustom,
		Active:   m.Name == current,
	}
}

// SwitchModel 切模型。
func (h *ModelHandler) SwitchModel(c *gin.Context) {
	var req struct {
		Model string `json:"model" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	if err := h.manager.Switch(req.Model); err != nil {
		h.logger.Error("switch model failed", zap.String("model", req.Model), zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(gin.H{"model": req.Model, "message": "switched successfully"}))
}

// AddCustomModel 加一个自定义模型。
func (h *ModelHandler) AddCustomModel(c *gin.Context) {
	var entry config.ModelEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	if entry.Name == "" || entry.ChatModel == "" {
		c.JSON(http.StatusBadRequest, model.Err(400, "name and chat_model are required"))
		return
	}
	if err := h.store.Add(entry); err != nil {
		h.logger.Warn("add custom model", zap.String("name", entry.Name), zap.Error(err))
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(gin.H{"name": entry.Name, "message": "model added"}))
}

// RemoveCustomModel 删一个自定义模型。
func (h *ModelHandler) RemoveCustomModel(c *gin.Context) {
	name := c.Param("name")
	if err := h.store.Remove(name); err != nil {
		h.logger.Warn("remove custom model", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}
	c.JSON(http.StatusOK, model.OK(gin.H{"name": name, "message": "model removed"}))
}
