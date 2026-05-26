package modelmanager

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/openaimodel"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// ModelManager implements model.ChatModel and supports runtime model switching
// via atomic.Value. Looks up models from config model_list AND custom ModelStore.
type ModelManager struct {
	current   atomic.Value // stores model.ChatModel
	cfg       *config.Config
	customStore *service.ModelStore
	logger    *zap.Logger
	tools     []*schema.ToolInfo
	name      atomic.Value
}

func NewModelManager(initial model.ChatModel, cfg *config.Config, customStore *service.ModelStore, resolvedModel, resolvedBaseURL string, logger *zap.Logger) *ModelManager {
	mm := &ModelManager{cfg: cfg, customStore: customStore, logger: logger}
	mm.current.Store(initial)
	for _, e := range cfg.ModelProvider.ModelList {
		if e.ChatModel == resolvedModel && e.BaseURL == resolvedBaseURL {
			mm.name.Store(e.Name)
			break
		}
	}
	if mm.name.Load() == nil {
		for _, e := range customStore.All() {
			if e.ChatModel == resolvedModel && e.BaseURL == resolvedBaseURL {
				mm.name.Store(e.Name)
				break
			}
		}
	}
	return mm
}

// Switch creates a new OpenAIChatModel from the named entry (searches config
// model_list first, then custom models), re-binds tools, and atomically swaps.
func (m *ModelManager) Switch(modelName string) error {
	entry := m.findEntry(modelName)
	if entry == nil {
		return fmt.Errorf("model %q not found", modelName)
	}

	newModel := openaimodel.NewOpenAIChatModel(entry.APIKey, entry.ChatModel, entry.BaseURL, m.logger)
	if len(m.tools) > 0 {
		if err := newModel.BindTools(m.tools); err != nil {
			return fmt.Errorf("bind tools to new model: %w", err)
		}
	}

	m.current.Store(newModel)
	m.name.Store(modelName)
	m.logger.Info("switched model", zap.String("model", modelName))
	return nil
}

func (m *ModelManager) findEntry(name string) *config.ModelEntry {
	for i := range m.cfg.ModelProvider.ModelList {
		if m.cfg.ModelProvider.ModelList[i].Name == name {
			return &m.cfg.ModelProvider.ModelList[i]
		}
	}
	for _, e := range m.customStore.All() {
		if e.Name == name {
			cp := e
			return &cp
		}
	}
	return nil
}

func (m *ModelManager) CurrentName() string {
	if v := m.name.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (m *ModelManager) cur() model.ChatModel {
	if v, ok := m.current.Load().(model.ChatModel); ok {
		return v
	}
	return nil
}

func (m *ModelManager) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.cur().Generate(ctx, input, opts...)
}

func (m *ModelManager) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.cur().Stream(ctx, input, opts...)
}

func (m *ModelManager) BindTools(tools []*schema.ToolInfo) error {
	m.tools = tools
	return m.cur().BindTools(tools)
}

var _ model.ChatModel = (*ModelManager)(nil)
