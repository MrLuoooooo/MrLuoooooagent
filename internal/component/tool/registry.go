package tool

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/schema"
	eino_tool "github.com/cloudwego/eino/components/tool"
)

// Tool is the interface a tool must implement to be registry-compatible.
// Matches eino's tool.InvokableTool signature.
type Tool interface {
	Info(ctx context.Context) (*schema.ToolInfo, error)
	InvokableRun(ctx context.Context, argumentsInJSON string, opts ...eino_tool.Option) (string, error)
}

// globalRegistry is the application-wide tool registry singleton.
var globalRegistry = &Registry{
	tools: make([]Tool, 0),
}

// Registry is a thread-safe container for Tool instances.
type Registry struct {
	mu    sync.Mutex
	tools []Tool
}

// Register adds a tool to the global registry.
func Register(t Tool) error {
	return globalRegistry.Add(t)
}

// RegisteredTools 返回全局注册的所有工具。
func RegisteredTools() []Tool {
	return globalRegistry.List()
}

// RegisteredToolsByNames 按工具名白名单过滤，用于子 agent 工具集隔离。
// 空白名单返回空切片；不存在的名字被忽略（调用方可据此判断配置错误）。
func RegisteredToolsByNames(names []string) []Tool {
	if len(names) == 0 {
		return nil
	}
	ctx := context.Background()
	all := globalRegistry.List()
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	out := make([]Tool, 0, len(names))
	for _, t := range all {
		info, err := t.Info(ctx)
		if err != nil {
			continue
		}
		if want[info.Name] {
			out = append(out, t)
		}
	}
	return out
}

// ToolInfos returns the ToolInfo schemas for all registered tools.
func ToolInfos(ctx context.Context) ([]*schema.ToolInfo, error) {
	return globalRegistry.Info(ctx)
}

// Add registers a tool.
func (r *Registry) Add(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = append(r.tools, t)
	return nil
}

// List 列出所有已注册工具。
func (r *Registry) List() []Tool {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Tool, len(r.tools))
	copy(result, r.tools)
	return result
}

// Info returns ToolInfo for all registered tools.
func (r *Registry) Info(ctx context.Context) ([]*schema.ToolInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	infos := make([]*schema.ToolInfo, 0, len(r.tools))
	for _, t := range r.tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}
