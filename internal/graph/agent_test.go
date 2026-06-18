package graph

import (
	"context"
	"strings"
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

	g, err := NewAgentGraph(cm, tn, nil, nil, nil, "", "", nil, NewRetryGate(0))
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

	g, err := NewAgentGraph(cm, tn, nil, nil, nil, "", "", nil, NewRetryGate(0))
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

// capturingChatModel records system+user messages passed to Generate,
// then delegates to the wrapped model for the response.
type capturingChatModel struct {
	wrapped  model.ChatModel
	messages []*schema.Message // captured on latest Generate call
}

func (c *capturingChatModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	c.messages = msgs
	return c.wrapped.Generate(ctx, msgs, opts...)
}

func (c *capturingChatModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	c.messages = msgs
	return c.wrapped.Stream(ctx, msgs, opts...)
}

func (c *capturingChatModel) BindTools(tools []*schema.ToolInfo) error {
	return c.wrapped.BindTools(tools)
}

func TestToolExecutionStrategy_Injected(t *testing.T) {
	inner := &fakeChatModel{
		responses: []*schema.Message{
			{Role: schema.Assistant, Content: "ok"},
		},
	}
	cap := &capturingChatModel{wrapped: inner}

	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cap, tn, nil, nil, nil, "you are a helpful assistant.", "", nil, NewRetryGate(0))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	_, err = g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "hello",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}

	// 验证 system message 包含并行调用策略
	foundSystem := false
	for _, msg := range cap.messages {
		if msg.Role == schema.System {
			foundSystem = true
			if !strings.Contains(msg.Content, "工具并行调用策略") {
				t.Errorf("system prompt missing toolExecutionStrategy:\n%s", msg.Content)
			}
			if !strings.Contains(msg.Content, "you are a helpful assistant") {
				t.Errorf("system prompt missing user-defined prompt:\n%s", msg.Content)
			}
		}
	}
	if !foundSystem {
		t.Error("no system message found in captured messages")
	}
}

func TestStockModePromptSwitch(t *testing.T) {
	inner := &fakeChatModel{
		responses: []*schema.Message{
			{Role: schema.Assistant, Content: "ok"},
		},
	}
	cap := &capturingChatModel{wrapped: inner}

	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	generalPrompt := "you are a general assistant."
	stockPrompt := "you are a stock analysis specialist. Analyze using MACD/RSI/BOLL."

	g, err := NewAgentGraph(cap, tn, nil, nil, nil, generalPrompt, stockPrompt, nil, NewRetryGate(0))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// 无 stock_mode → 通用 prompt
	_, err = g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "hello",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	for _, msg := range cap.messages {
		if msg.Role == schema.System && !strings.Contains(msg.Content, generalPrompt) {
			t.Errorf("without stock_mode, expected general prompt. got:\n%s", msg.Content)
		}
	}

	// stock_mode=true → 股票 prompt
	ctx := context.WithValue(context.Background(), "stock_mode", true)
	_, err = g.Invoke(ctx, &schema.Message{
		Role: schema.User, Content: "analyze this stock",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	foundStock := false
	for _, msg := range cap.messages {
		if msg.Role == schema.System {
			foundStock = true
			if !strings.Contains(msg.Content, stockPrompt) {
				t.Errorf("with stock_mode=true, expected stock prompt. got:\n%s", msg.Content)
			}
			if strings.Contains(msg.Content, generalPrompt) {
				t.Errorf("with stock_mode=true, should NOT contain general prompt. got:\n%s", msg.Content)
			}
		}
	}
	if !foundStock {
		t.Error("no system message with stock_mode")
	}

	// stock_mode=false → 通用 prompt
	ctx = context.WithValue(context.Background(), "stock_mode", false)
	_, err = g.Invoke(ctx, &schema.Message{
		Role: schema.User, Content: "hello again",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	for _, msg := range cap.messages {
		if msg.Role == schema.System && !strings.Contains(msg.Content, generalPrompt) {
			t.Errorf("with stock_mode=false, expected general prompt. got:\n%s", msg.Content)
		}
	}
}

