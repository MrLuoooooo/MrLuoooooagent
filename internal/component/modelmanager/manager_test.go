package modelmanager

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// testModel implements model.ChatModel for testing.
type testModel struct {
	name string
}

func (m *testModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "from " + m.name}, nil
}

func (m *testModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, _ := m.Generate(ctx, msgs, opts...)
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (m *testModel) BindTools(tools []*schema.ToolInfo) error { return nil }

func TestModelManager_Generate(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{}
	store := service.NewModelStore("data")

	mm := NewModelManager(&testModel{name: "default"}, cfg, store, "", "", logger)

	msg, err := mm.Generate(context.Background(), []*schema.Message{
		{Role: schema.User, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if msg.Content != "from default" {
		t.Errorf("unexpected content: %s", msg.Content)
	}
}

func TestModelManager_CurrentName_Empty(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{}
	store := service.NewModelStore("data")

	mm := NewModelManager(&testModel{name: "default"}, cfg, store, "", "", logger)
	if name := mm.CurrentName(); name != "" {
		t.Errorf("expected empty name, got %q", name)
	}
}

func TestModelManager_BindTools_Propagates(t *testing.T) {
	logger := zap.NewNop()
	cfg := &config.Config{}
	store := service.NewModelStore("data")

	mm := NewModelManager(&testModel{name: "default"}, cfg, store, "", "", logger)

	tools := []*schema.ToolInfo{{Name: "test_tool"}}
	if err := mm.BindTools(tools); err != nil {
		t.Fatalf("BindTools() error = %v", err)
	}
	// tools stored internally (verified via successful call, no panic)
}
