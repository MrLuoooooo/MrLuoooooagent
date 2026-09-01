package callback

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	tool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ToolRunRecord 一次工具调用的完整记录：调用信息 + 结果文本。
// 由 callback 在工具节点 OnStart/OnEnd 分别填充，供 SSE 来源提取与置信度评估消费。
type ToolRunRecord struct {
	ToolCallID string // 可能缺失（eino CallbackInput 不携带）
	ToolName   string
	Args       string
	Result     string
}

// ToolResultsBag 跨 callback/caller 共享的工具调用收集器。
//
// 注意：eino 的 tool.CallbackInput 不携带 ToolCallID，且 ToolsNode 并发执行
// 多工具调用时 OnStart/OnEnd 可能交错——Result 按"第一个空位"回填，
// 单工具轮次精确，多工具轮次可能互换，对来源归因场景足够。
type ToolResultsBag struct {
	mu      sync.Mutex
	Results []*schema.Message // 兼容旧消费方（置信度 FactCheck）
	Records []ToolRunRecord   // 结构化记录（来源提取用）
}

// FillResult 回填结果到第一个空 Result 的记录；没有可回填记录时兜底新建。
func (b *ToolResultsBag) FillResult(toolCallID, content string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if toolCallID != "" {
		for i := range b.Records {
			if b.Records[i].ToolCallID == toolCallID && b.Records[i].Result == "" {
				b.Records[i].Result = content
				return
			}
		}
	}
	for i := range b.Records {
		if b.Records[i].Result == "" {
			b.Records[i].Result = content
			return
		}
	}
	b.Records = append(b.Records, ToolRunRecord{ToolCallID: toolCallID, Result: content})
}

// appendMsg 旧语义兼容：收集 Tool 消息 + 同步回填 Records。
func (b *ToolResultsBag) appendMsg(m *schema.Message) {
	b.mu.Lock()
	b.Results = append(b.Results, m)
	b.mu.Unlock()
	b.FillResult(m.ToolCallID, m.Content)
}

// appendRecord 追加一条调用记录（OnStart 阶段，工具名优先取 RunInfo）。
func (b *ToolResultsBag) appendRecord(name, args string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Records = append(b.Records, ToolRunRecord{ToolName: name, Args: args})
}

// NewToolCollector 建一个 Callback，拦截 Tool 类型组件：
// OnStart 抓工具名与参数（eino tool.CallbackInput / string / Message 三种入口都处理），
// OnEnd 抓结果文本，填入 bag。
// 调用方注入 compose.WithCallbacks 后，Agent 执行完毕从 bag 读取。
func NewToolCollector(bag *ToolResultsBag) callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			if info == nil || info.Component != components.ComponentOfTool {
				return ctx
			}
			switch v := input.(type) {
			case *tool.CallbackInput:
				// ToolsNode 标准入口：参数是原始 JSON 字符串，工具名取 RunInfo.Name
				bag.appendRecord(info.Name, v.ArgumentsInJSON)
			case string:
				// 部分路径直接传字符串参数
				bag.appendRecord(info.Name, v)
			case *schema.Message:
				bag.startFromMessage(info.Name, v.ToolCalls)
			case []*schema.Message:
				for _, m := range v {
					bag.startFromMessage(info.Name, m.ToolCalls)
				}
			default:
				// 拿不到参数也要记一条占位，保证 Result 有地方落
				bag.appendRecord(info.Name, "")
			}
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			if info == nil || info.Component != components.ComponentOfTool {
				return ctx
			}
			switch v := output.(type) {
			case *tool.CallbackOutput:
				bag.appendMsg(&schema.Message{Role: schema.Tool, Content: v.Response})
			case *schema.Message:
				bag.appendMsg(v)
			case []*schema.Message:
				for _, m := range v {
					bag.appendMsg(m)
				}
			case string:
				bag.appendMsg(&schema.Message{Role: schema.Tool, Content: v})
			}
			return ctx
		}).
		Build()
}

// startFromMessage Message 入口：从 ToolCalls 展开记录（优先用调用自带的名称/参数）。
func (b *ToolResultsBag) startFromMessage(fallbackName string, calls []schema.ToolCall) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, tc := range calls {
		name := tc.Function.Name
		if name == "" {
			name = fallbackName
		}
		b.Records = append(b.Records, ToolRunRecord{
			ToolCallID: tc.ID,
			ToolName:   name,
			Args:       tc.Function.Arguments,
		})
	}
	if len(calls) == 0 {
		b.Records = append(b.Records, ToolRunRecord{ToolName: fallbackName})
	}
}
