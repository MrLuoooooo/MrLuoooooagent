package modelmanager

import (
	"context"
	"testing"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type fakeChatModel struct{}

func (fakeChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return nil, nil
}

func (fakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (fakeChatModel) BindTools(tools []*schema.ToolInfo) error { return nil }
func (fakeChatModel) GetType() string                          { return "fake" }

func TestDerive_ByExplicitName(t *testing.T) {
	cfg := &config.Config{
		ModelProvider: config.ModelProviderConfig{
			ModelList: []config.ModelEntry{
				{Name: "deepseek", ChatModel: "deepseek-chat", BaseURL: "https://api.deepseek.com/v1", APIKey: "k"},
				{Name: "ollama", ChatModel: "qwen3.5:9b", BaseURL: "http://localhost:11434", APIKey: ""},
			},
		},
	}
	mm := NewModelManager(fakeChatModel{}, cfg, service.NewModelStore(""), "deepseek-chat", "https://api.deepseek.com/v1", zap.NewNop())

	m, err := mm.Derive("ollama")
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if m == nil {
		t.Fatal("derive returned nil model")
	}
}

func TestDerive_UnknownModel(t *testing.T) {
	cfg := &config.Config{ModelProvider: config.ModelProviderConfig{}}
	mm := NewModelManager(fakeChatModel{}, cfg, service.NewModelStore(""), "", "", zap.NewNop())
	if _, err := mm.Derive("nonexistent"); err == nil {
		t.Fatal("want error for unknown model")
	}
}
