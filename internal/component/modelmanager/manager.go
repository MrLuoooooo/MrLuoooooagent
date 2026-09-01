package modelmanager

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/openaimodel"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// CustomModelLister 是本包对自定义模型存储的最小依赖（消费方定义接口）。
// service.ModelStore 天然满足，生产代码不 import service——component 不得反向依赖 service 层。
type CustomModelLister interface {
	All() []config.ModelEntry
}

// ModelManager包装ChatModel支持运行时切换模型。
// 通过atomic.Value实现，从configmodel_list和customModelStore中查找模型。
type ModelManager struct {
	current     atomic.Value // stores model.ChatModel
	cfg         *config.Config
	customStore CustomModelLister
	logger      *zap.Logger
	tools       []*schema.ToolInfo
	name        atomic.Value
}

func NewModelManager(initial model.ChatModel, cfg *config.Config, customStore CustomModelLister, resolvedModel, resolvedBaseURL string, logger *zap.Logger) *ModelManager {
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

// contextWindows 常见模型的上下文窗口映射（token 数）。
// 未命中映射的自定义/本地模型走 defaultContextWindow——宁保守不大算错，
// 预算偏小只多触发一次摘要，预算偏大会撑爆模型上下文。
var contextWindows = map[string]int{
	"deepseek-chat":     64 * 1024,
	"deepseek-reasoner": 64 * 1024,
	"qwen-plus":         128 * 1024,
	"qwen-turbo":        128 * 1024,
	"qwen-max":          32 * 1024,
	"gpt-4o":            128 * 1024,
	"gpt-4o-mini":       128 * 1024,
}

const defaultContextWindow = 32 * 1024

// ContextWindow 返回当前模型的上下文窗口大小（token）。
// 短期记忆 token 预算裁剪（service.TrimHistory）的预算来源。
func (m *ModelManager) ContextWindow() int {
	if w, ok := contextWindows[m.CurrentName()]; ok {
		return w
	}
	return defaultContextWindow
}

// Switch 按名字切到新模型（先查 config model_list，再查自定义模型），重新 bind tools 后原子替换。
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

// Derive 创建与指定模型同 provider 的独立 ChatModel 实例，用于子 agent。
// 与主 agent 共享 ModelManager 时 BindTools 会互相覆盖（主 agent 绑全量、
// 子 agent 绑子集），Derive 返回全新实例，绑定互不干扰。
// modelName 为空时使用当前模型；不 BindTools，由调用方自行绑定。
func (m *ModelManager) Derive(modelName string) (model.ChatModel, error) {
	if modelName == "" {
		modelName = m.CurrentName()
	}
	entry := m.findEntry(modelName)
	if entry == nil {
		return nil, fmt.Errorf("model %q not found", modelName)
	}
	return openaimodel.NewOpenAIChatModel(entry.APIKey, entry.ChatModel, entry.BaseURL, m.logger), nil
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
