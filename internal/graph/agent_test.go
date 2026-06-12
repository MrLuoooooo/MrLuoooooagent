package graph

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	eino_tool "github.com/cloudwego/eino/components/tool"
)

// fakeChatModel implements model.ChatModel for testing graph compilation and branching.
type fakeChatModel struct {
	responses []*schema.Message
	callCount int
}

func (f *fakeChatModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	f.callCount++
	if len(f.responses) > 0 {
		idx := min(f.callCount-1, len(f.responses)-1)
		return f.responses[idx], nil
	}
	return &schema.Message{Role: schema.Assistant, Content: "ok"}, nil
}

func (f *fakeChatModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := f.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (f *fakeChatModel) BindTools(tools []*schema.ToolInfo) error { return nil }

func TestNewAgentGraph_Compiles(t *testing.T) {
	cm := &fakeChatModel{}
	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cm, tn, nil, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("NewAgentGraph() error = %v", err)
	}
	if g == nil {
		t.Fatal("graph should not be nil")
	}
}

func TestNewAgentGraph_DirectAnswer(t *testing.T) {
	cm := &fakeChatModel{
		responses: []*schema.Message{
			{Role: schema.Assistant, Content: "hello world"},
		},
	}
	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cm, tn, nil, nil, nil, "", nil)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "hi",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Content != "hello world" {
		t.Errorf("content = %q, want %q", result.Content, "hello world")
	}
}

