package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/tool"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// StockModeKey 股票专精模式 context key，类型安全避免跨包冲突。
type contextKey string
const StockModeKey contextKey = "stock_mode"

// AgentGraphOptions 控制 agent 图构造行为。
// 主 agent 用默认值（全开）；子 agent 关闭与自身无关的上下文注入，
// 避免共享全局状态（checkpoint/retrygate 由调用方保证独立实例）。
type AgentGraphOptions struct {
	SystemPrompt       string
	StockPrompt        string
	DisableCheckpoint  bool // 子 agent 必须为 true：断点恢复只对主 agent 有意义
	DisableMemory      bool
	DisableSkills      bool
	DisableMcp         bool
	DisableWorkspace   bool
	DisableQueryRewrite bool
	MaxRunSteps        int  // ≤0 时用默认 200
}

// NewAgentGraph 兼容旧签名：等价于 NewAgentGraphWithOptions 全默认值。
func NewAgentGraph(
	chatModel model.ChatModel,
	toolsNode *compose.ToolsNode,
	toolInfos []*schema.ToolInfo,
	skills *service.SkillStore,
	memorySvc *service.MemoryService,
	mcpStore *service.McpStore,
	systemPrompt string,
	stockSystemPrompt string,
	cpStore *store.CheckpointStore,
	retryGate *RetryGate,
) (compose.Runnable[*schema.Message, *schema.Message], error) {
	return NewAgentGraphWithOptions(chatModel, toolsNode, toolInfos, skills, memorySvc, mcpStore, AgentGraphOptions{
		SystemPrompt: systemPrompt,
		StockPrompt:  stockSystemPrompt,
	}, cpStore, retryGate)
}

// NewAgentGraphWithOptions 构建 agent 图，支持子 agent 场景的上下文裁剪。
func NewAgentGraphWithOptions(
	chatModel model.ChatModel,
	toolsNode *compose.ToolsNode,
	toolInfos []*schema.ToolInfo,
	skills *service.SkillStore,
	memorySvc *service.MemoryService,
	mcpStore *service.McpStore,
	opts AgentGraphOptions,
	cpStore *store.CheckpointStore,
	retryGate *RetryGate,
) (compose.Runnable[*schema.Message, *schema.Message], error) {

	sysPrompt := opts.SystemPrompt
	stockPrompt := opts.StockPrompt

	graph := compose.NewGraph[*schema.Message, *schema.Message]()

	graph.AddLambdaNode("to_messages", compose.InvokableLambda(
		func(ctx context.Context, msg *schema.Message) ([]*schema.Message, error) {
			prompt := sysPrompt
			// 股票专精模式：若 context 中有 stock_mode=true 且有股票专用 prompt，则使用之。
			if v, ok := ctx.Value(StockModeKey).(bool); ok && v && stockPrompt != "" {
				prompt = stockPrompt
			}
			// 注入用户长期记忆
			if !opts.DisableMemory && memorySvc != nil {
				memBlock := memorySvc.InjectIntoPrompt(ctx, "default", msg.Content)
				if memBlock != "" {
					prompt += memBlock
				}
			}
			if !opts.DisableSkills && skills != nil {
				enabled := skills.Enabled()
				if len(enabled) > 0 {
					var sb strings.Builder
					sb.WriteString("\n## 用户自定义技能\n")
					for _, s := range enabled {
						sb.WriteString("\n### ")
						sb.WriteString(s.Name)
						sb.WriteString("\n")
						sb.WriteString(s.Prompt)
						sb.WriteString("\n")
					}
					prompt += sb.String()
				}
			}
			// 注入工具并行调用策略（不修改用户 system_prompt，以扩展方式附加）
			prompt += toolExecutionStrategy
			// Inject current workspace before tools section so the model sees it first.
			if !opts.DisableWorkspace {
				win := tool.GetWorkspaceWinPath()
				if win == "" {
					win = tool.GetWorkspaceRoot()
				}
				if win != "" {
					summary := tool.ReadWorkspaceSummary()
					wsBlock := "\n## 当前工作目录\n路径: " + win + "\n目录内容:\n" + summary +
						"\n用户询问工作目录时直接回答以上信息，不要调用任何工具。"
					prompt = wsBlock + "\n" + prompt
				}
			}
			// 注入用户上传的 MCP 项目列表
			if !opts.DisableMcp && mcpStore != nil {
				servers, _ := mcpStore.Load()
				if len(servers) > 0 {
					var sb strings.Builder
					sb.WriteString("\n## 用户上传的 MCP 项目\n")
					for _, s := range servers {
						sb.WriteString("- ")
						sb.WriteString(s.Name)
						sb.WriteString(" (transport=")
						sb.WriteString(s.Transport)
						sb.WriteString(")")
						if s.URL != "" {
							sb.WriteString(" 路径: ")
							sb.WriteString(s.URL)
						}
						if s.Transport == "agent" {
							sb.WriteString(" 【agent-managed，可直接读取文件并按需改造】")
						} else if s.Transport == "stdio" {
							sb.WriteString(fmt.Sprintf(" 命令: %s %v", s.Command, s.Args))
						}
						sb.WriteString("\n")
					}
					prompt += sb.String()
				}
			}
			// Query rewrite: 口语化/错别字/缩写归一为书面提问
			userContent := msg.Content
			if !opts.DisableQueryRewrite {
				rewritten := rewriteQueryIfNeeded(ctx, chatModel, userContent)
				if rewritten != "" {
					userContent = rewritten
				}
			}

			return []*schema.Message{
				{Role: schema.System, Content: prompt},
				{Role: schema.User, Content: prefixForContent(userContent)},
			}, nil
		},
	))

	graph.AddChatModelNode("chat_model", chatModel)

	// parse_tool_calls 用 TransformableLambda 而非 InvokableLambda：
	// Invokable 会把上游 chat_model 的流整体合并成单条消息，最终回答失去流式
	// （表现为前端"一瞬间"收到全文）。Transform 逐 chunk 透传内容保持流式；
	// ToolCalls 聚合完成后单条补发，保证下游 branch 合并路由与 handler
	// 收到完整工具调用参数（与旧单条行为一致）。
	graph.AddLambdaNode("parse_tool_calls", compose.TransformableLambda(
		func(ctx context.Context, sr *schema.StreamReader[*schema.Message]) (*schema.StreamReader[*schema.Message], error) {
			pr, pw := schema.Pipe[*schema.Message](32)
			go func() {
				defer pw.Close()
				send := func(msg *schema.Message) bool {
					if closed := pw.Send(msg, nil); closed {
						sr.Close() // 下游提前关闭，释放上游流
						return true
					}
					return false
				}
				var chunks []*schema.Message
				for {
					chunk, err := sr.Recv()
					if err != nil {
						break
					}
					chunks = append(chunks, chunk)
					// 内容原样透传（handler 侧 stripToolCode 负责过滤工具块）；
					// ToolCalls 置空副本透传，聚合版延后单条发，避免增量分片导致事件参数残缺
					cp := *chunk
					cp.ToolCalls = nil
					if send(&cp) {
						return
					}
				}
				if len(chunks) == 0 {
					return
				}
				agg, err := schema.ConcatMessages(chunks)
				if err != nil {
					return
				}
				// 原生函数调用：chunk 里的增量分片聚合为完整 ToolCalls
				// XML 文本格式兜底：不支持原生函数调用的模型用 <tool_code> 块表达意图
				if len(agg.ToolCalls) == 0 {
					agg.ToolCalls = parsePromptToolCalls(agg.Content)
				}
				if len(agg.ToolCalls) > 0 {
					agg.Content = "" // 内容已随 chunk 透传，置空防止 branch 合并时重复
					send(agg)
				}
			}()
			return pr, nil
		},
	))

	graph.AddToolsNode("tools", toolsNode)

	graph.AddLambdaNode("retry_gate", compose.InvokableLambda(retryGate.Intercept))

	graph.AddLambdaNode("tool_as_user", compose.InvokableLambda(
		func(ctx context.Context, msgs []*schema.Message) ([]*schema.Message, error) {
			out := make([]*schema.Message, len(msgs))
			for i, m := range msgs {
				out[i] = &schema.Message{Role: schema.User, Content: "操作完成：" + m.Content + " 请基于以上结果对用户做出简短回复。"}
			}
			return out, nil
		},
	))

	graph.AddEdge(compose.START, "to_messages")
	graph.AddEdge("to_messages", "chat_model")
	graph.AddEdge("chat_model", "parse_tool_calls")

	graph.AddBranch("parse_tool_calls", compose.NewGraphBranch(
		func(ctx context.Context, msg *schema.Message) (string, error) {
			if len(msg.ToolCalls) > 0 {
				return "tools", nil
			}
			return compose.END, nil
		},
		map[string]bool{"tools": true, compose.END: true},
	))

	graph.AddEdge("tools", "retry_gate")
	graph.AddEdge("retry_gate", "tool_as_user")
	graph.AddEdge("tool_as_user", "chat_model")

	// 使用 Eino 内置的 checkpoint 机制（子 agent 必须禁用，避免断点状态串扰）
	compileOpts := []compose.GraphCompileOption{compose.WithMaxRunSteps(200)}
	if opts.MaxRunSteps > 0 {
		compileOpts = []compose.GraphCompileOption{compose.WithMaxRunSteps(opts.MaxRunSteps)}
	}
	if !opts.DisableCheckpoint && cpStore != nil {
		compileOpts = append(compileOpts, compose.WithCheckPointStore(cpStore))
	}

	return graph.Compile(context.Background(), compileOpts...)
}

// prefixForContent detects write/create intent and guides tool usage.
func prefixForContent(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "写") || strings.Contains(lower, "创建") || strings.Contains(lower, "write") || strings.Contains(lower, "create") {
		return "TASK: " + content + " (Use write_file tool now. Do not list files or describe workspace.)"
	}
	return content
}

// toolExecutionStrategy 并行工具调用指导规则。
// 以扩展方式注入 prompt，不修改用户配置的 system_prompt 原文。
const toolExecutionStrategy = `

## 工具并行调用策略

当单次回复中需要调用多个互不依赖的工具时，在同一轮中一次性发出所有 tool_calls，例如：
- 同时查询多只股票行情（get_stock_realtime 支持批量，但多只独立查询也可并行）
- 同时搜索多个不相关的关键词
- 同时读取多个不同文件

这些工具调用将并行执行，大幅减少等待时间。

仅在以下情况才分步调用：
- 后一个工具的输入依赖前一个工具的输出
- 同一个文件需要先写入再读取
- 需要根据文件列表的结果决定下一步操作
`
