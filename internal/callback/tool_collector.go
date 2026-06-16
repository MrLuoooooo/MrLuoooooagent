package callback

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
)

// ToolResultsBag 跨 callback/caller 共享的工具结果收集器。
type ToolResultsBag struct {
	mu      sync.Mutex
	Results []*schema.Message
}

// NewToolCollector 建一个 Callback，拦截 Tool 类型节点输出并收集到 bag。
// 调用方注入 compose.WithCallbacks 后，Agent 执行完毕从 bag.Results 读取。
func NewToolCollector(bag *ToolResultsBag) callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			if info == nil || info.Component != components.ComponentOfTool {
				return ctx
			}
			switch v := output.(type) {
			case *schema.Message:
				bag.mu.Lock()
				bag.Results = append(bag.Results, v)
				bag.mu.Unlock()
			case []*schema.Message:
				bag.mu.Lock()
				bag.Results = append(bag.Results, v...)
				bag.mu.Unlock()
			case string:
				// 单工具直接返回 string 的情况，包装成 Tool 消息
				bag.mu.Lock()
				bag.Results = append(bag.Results, &schema.Message{Role: schema.Tool, Content: v})
				bag.mu.Unlock()
			}
			return ctx
		}).
		Build()
}
