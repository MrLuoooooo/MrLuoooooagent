package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	eino_tool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ——— mock tool ———

type mockTool struct {
	name    string
	desc    string
	handler func(args string) string
}

func (m *mockTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: m.name, Desc: m.desc}, nil
}

func (m *mockTool) InvokableRun(ctx context.Context, args string, opts ...eino_tool.Option) (string, error) {
	return m.handler(args), nil
}

var _ eino_tool.InvokableTool = (*mockTool)(nil)

// ——— mock LLM that returns tool_calls ———

type toolCallingModel struct {
	callCount  int
	responses  []*schema.Message
}

func (f *toolCallingModel) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	f.callCount++
	if f.callCount <= len(f.responses) {
		return f.responses[f.callCount-1], nil
	}
	return &schema.Message{Role: schema.Assistant, Content: "done"}, nil
}

func (f *toolCallingModel) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, err := f.Generate(ctx, msgs, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (f *toolCallingModel) BindTools(tools []*schema.ToolInfo) error { return nil }

// ——— helpers ———

var callID int

func toolCall(name, args string) *schema.Message {
	callID++
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID: fmt.Sprintf("c_%d", callID),
			Function: schema.FunctionCall{
				Name:      name,
				Arguments: args,
			},
		}},
	}
}

// ——— tests ———

func TestAgent_ReActLoopWithToolCall(t *testing.T) {
	cm := &toolCallingModel{
		responses: []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID:   "call_1",
						Function: schema.FunctionCall{
							Name:      "get_stock_price",
							Arguments: `{"symbol":"600519"}`,
						},
					},
				},
			},
			{Role: schema.Assistant, Content: "茅台当前股价 1800 元"},
		},
	}

	priceTool := &mockTool{
		name: "get_stock_price",
		desc: "查询股票实时价格",
		handler: func(args string) string {
			return "600519 最新价: 1800.00"
		},
	}

	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{priceTool},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cm, tn, []*schema.ToolInfo{{
		Name: "get_stock_price",
		Desc: "查询股票实时价格",
	}}, nil, nil, nil, "你是金融分析助手", "", nil, NewRetryGate(3))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "茅台现在多少钱",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if cm.callCount != 2 {
		t.Fatalf("expected 2 LLM calls (tool_call + response), got %d", cm.callCount)
	}
	if !strings.Contains(result.Content, "1800") {
		t.Fatalf("response should contain stock price, got: %s", result.Content)
	}
}

func TestAgent_ParseToolCallsFromXML(t *testing.T) {
	cm := &toolCallingModel{
		responses: []*schema.Message{
			{
				Role:    schema.Assistant,
				Content: `<tool_call>{"name":"echo","arguments":{"text":"hello"}}</tool_call>`,
			},
			{Role: schema.Assistant, Content: "工具返回了 hello"},
		},
	}

	echoTool := &mockTool{
		name: "echo",
		desc: "echo text",
		handler: func(args string) string {
			var v map[string]any
			json.Unmarshal([]byte(args), &v)
			return v["text"].(string)
		},
	}

	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{echoTool},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cm, tn, []*schema.ToolInfo{{
		Name: "echo",
		Desc: "echo text",
	}}, nil, nil, nil, "你是助手", "", nil, NewRetryGate(3))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "echo hello",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if cm.callCount != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", cm.callCount)
	}
	if !strings.Contains(result.Content, "hello") {
		t.Fatalf("response should contain echoed text, got: %s", result.Content)
	}
}

func TestAgent_DirectAnswer_NoToolCall(t *testing.T) {
	cm := &toolCallingModel{
		responses: []*schema.Message{
			{Role: schema.Assistant, Content: "你好，有什么可以帮你的？"},
		},
	}

	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cm, tn, nil, nil, nil, nil, "你是助手", "", nil, NewRetryGate(3))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "你好",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if cm.callCount != 1 {
		t.Fatalf("should only call LLM once (no tool), got %d", cm.callCount)
	}
	if !strings.Contains(result.Content, "你好") {
		t.Fatalf("response wrong: %s", result.Content)
	}
}

func TestAgent_MultipleToolCalls(t *testing.T) {
	cm := &toolCallingModel{
		responses: []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					{
						ID: "c1",
						Function: schema.FunctionCall{
							Name:      "get_price",
							Arguments: `{"symbol":"600519"}`,
						},
					},
					{
						ID: "c2",
						Function: schema.FunctionCall{
							Name:      "get_revenue",
							Arguments: `{"symbol":"600519","year":"2025"}`,
						},
					},
				},
			},
			{Role: schema.Assistant, Content: "茅台股价 1800，营收 1234 亿"},
		},
	}

	priceTool := &mockTool{
		name: "get_price",
		desc: "查股价",
		handler: func(args string) string { return "股价 1800" },
	}
	revenueTool := &mockTool{
		name: "get_revenue",
		desc: "查营收",
		handler: func(args string) string { return "营收 1234亿" },
	}

	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{priceTool, revenueTool},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cm, tn, []*schema.ToolInfo{
		{Name: "get_price", Desc: "查股价"},
		{Name: "get_revenue", Desc: "查营收"},
	}, nil, nil, nil, "你是分析助手", "", nil, NewRetryGate(3))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "茅台股价和营收",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if cm.callCount != 2 {
		t.Fatalf("expected 2 LLM calls, got %d", cm.callCount)
	}
	if !strings.Contains(result.Content, "1800") || !strings.Contains(result.Content, "1234") {
		t.Fatalf("response should contain both tool results, got: %s", result.Content)
	}
}

func TestAgent_RetryGateBlocksRepeatedParamError(t *testing.T) {
	cm := &toolCallingModel{
		responses: []*schema.Message{
			// 连续 4 次参数错误触发 retry_gate 拦截
			toolCall("search", `{"keyword":""}`),
			toolCall("search", `{"keyword":""}`),
			toolCall("search", `{"keyword":""}`),
			toolCall("search", `{"keyword":""}`),
			// 被拦截后 LLM 生成终止回复
			{Role: schema.Assistant, Content: "无法搜索，参数持续错误"},
		},
	}

	failTool := &mockTool{
		name: "search",
		desc: "搜索",
		handler: func(args string) string {
			return "参数解析失败: keyword 不能为空"
		},
	}

	tn, err := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{failTool},
	})
	if err != nil {
		t.Fatalf("create tool node: %v", err)
	}

	g, err := NewAgentGraph(cm, tn, []*schema.ToolInfo{
		{Name: "search", Desc: "搜索"},
	}, nil, nil, nil, "你是助手", "", nil, NewRetryGate(3))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := g.Invoke(context.Background(), &schema.Message{
		Role: schema.User, Content: "搜索",
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if cm.callCount != 5 {
		t.Fatalf("expected 5 LLM calls (4 errors + 1 final), got %d", cm.callCount)
	}
	if !strings.Contains(result.Content, "无法搜索") {
		t.Fatalf("response should indicate search failure, got: %s", result.Content)
	}
}

// ——— Benchmark ———

func BenchmarkAgentGraph_ToolCalling(b *testing.B) {
	cm := &toolCallingModel{
		responses: []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID: "c1",
					Function: schema.FunctionCall{
						Name:      "echo",
						Arguments: `{"text":"hello"}`,
					},
				}},
			},
			{Role: schema.Assistant, Content: "ok"},
		},
	}

	echoTool := &mockTool{
		name: "echo",
		desc: "echo",
		handler: func(args string) string { return "hello" },
	}

	tn, _ := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{echoTool},
	})

	g, _ := NewAgentGraph(cm, tn, []*schema.ToolInfo{
		{Name: "echo", Desc: "echo"},
	}, nil, nil, nil, "", "", nil, NewRetryGate(3))

	msg := &schema.Message{Role: schema.User, Content: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cm.callCount = 0
		_, _ = g.Invoke(context.Background(), msg)
	}
}
