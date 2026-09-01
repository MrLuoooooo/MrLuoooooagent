package callback

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	tool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

func toolRunInfo() *callbacks.RunInfo {
	return &callbacks.RunInfo{Component: components.ComponentOfTool, Name: "get_kline"}
}

func TestToolCollector_EinoCallbackInput(t *testing.T) {
	// ToolsNode 标准入口：OnStart 收 tool.CallbackInput（参数 JSON），OnEnd 收 tool.CallbackOutput
	bag := &ToolResultsBag{}
	h := NewToolCollector(bag)
	ctx := context.Background()

	h.OnStart(ctx, toolRunInfo(), &tool.CallbackInput{ArgumentsInJSON: `{"code":"600519","period":"day"}`})
	h.OnEnd(ctx, toolRunInfo(), &tool.CallbackOutput{Response: `{"close":1520.5}`})

	if len(bag.Records) != 1 {
		t.Fatalf("want 1 record, got %d", len(bag.Records))
	}
	rec := bag.Records[0]
	if rec.ToolName != "get_kline" {
		t.Errorf("tool name should come from RunInfo, got %q", rec.ToolName)
	}
	if rec.Args != `{"code":"600519","period":"day"}` {
		t.Errorf("args should be captured: %q", rec.Args)
	}
	if rec.Result != `{"close":1520.5}` {
		t.Errorf("result should be captured: %q", rec.Result)
	}
}

func TestToolCollector_CallAndResult(t *testing.T) {
	bag := &ToolResultsBag{}
	h := NewToolCollector(bag)
	ctx := context.Background()

	// OnStart：assistant 消息携带 ToolCalls
	startInput := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "stock_kline",
				Arguments: `{"code":"600519"}`,
			},
		}},
	}
	h.OnStart(ctx, toolRunInfo(), startInput)

	// OnEnd：工具返回结果消息（带 ToolCallID）
	endOutput := &schema.Message{Role: schema.Tool, ToolCallID: "call_1", Content: `{"close":1299.56}`}
	h.OnEnd(ctx, toolRunInfo(), endOutput)

	if len(bag.Records) != 1 {
		t.Fatalf("want 1 record, got %d", len(bag.Records))
	}
	rec := bag.Records[0]
	if rec.ToolCallID != "call_1" || rec.ToolName != "stock_kline" || rec.Args != `{"code":"600519"}` {
		t.Errorf("record call info wrong: %+v", rec)
	}
	if rec.Result != `{"close":1299.56}` {
		t.Errorf("record result wrong: %q", rec.Result)
	}
	if len(bag.Results) != 1 || bag.Results[0].Content != `{"close":1299.56}` {
		t.Errorf("legacy Results broken: %+v", bag.Results)
	}
}

func TestToolCollector_NonToolIgnored(t *testing.T) {
	bag := &ToolResultsBag{}
	h := NewToolCollector(bag)
	ctx := context.Background()
	info := &callbacks.RunInfo{Component: components.ComponentOfChatModel}
	h.OnStart(ctx, info, &schema.Message{Role: schema.User, Content: "hi"})
	h.OnEnd(ctx, info, &schema.Message{Role: schema.Assistant, Content: "answer"})
	if len(bag.Records) != 0 || len(bag.Results) != 0 {
		t.Errorf("non-tool component should be ignored, got %+v", bag.Records)
	}
}

func TestToolCollector_StringResultFallback(t *testing.T) {
	bag := &ToolResultsBag{}
	h := NewToolCollector(bag)
	ctx := context.Background()
	// OnStart 未捕获到 ToolCalls（string 入口）→ 占位记录
	h.OnStart(ctx, toolRunInfo(), "raw input")
	// OnEnd 返回 string → 回填到占位记录
	h.OnEnd(ctx, toolRunInfo(), "tool output text")

	if len(bag.Records) != 1 {
		t.Fatalf("want 1 record, got %d", len(bag.Records))
	}
	if bag.Records[0].Result != "tool output text" || bag.Records[0].ToolName != "get_kline" {
		t.Errorf("fallback record wrong: %+v", bag.Records[0])
	}
}

func TestFillResult_NoRecordFallback(t *testing.T) {
	bag := &ToolResultsBag{}
	bag.FillResult("call_x", "orphan result")
	if len(bag.Records) != 1 || bag.Records[0].Result != "orphan result" {
		t.Errorf("orphan fill should create record, got %+v", bag.Records)
	}
}
