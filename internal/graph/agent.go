package graph

import (
	"context"
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

func NewAgentGraph(
	chatModel model.ChatModel,
	toolsNode *compose.ToolsNode,
	toolInfos []*schema.ToolInfo,
	skills *service.SkillStore,
	memorySvc *service.MemoryService,
	systemPrompt string,
	stockSystemPrompt string,
	cpStore *store.CheckpointStore,
	retryGate *RetryGate,
) (compose.Runnable[*schema.Message, *schema.Message], error) {

	sysPrompt := systemPrompt
	stockPrompt := stockSystemPrompt

	graph := compose.NewGraph[*schema.Message, *schema.Message]()

	graph.AddLambdaNode("to_messages", compose.InvokableLambda(
		func(ctx context.Context, msg *schema.Message) ([]*schema.Message, error) {
			prompt := sysPrompt
			// 股票专精模式：若 context 中有 stock_mode=true 且有股票专用 prompt，则使用之。
			if v, ok := ctx.Value(StockModeKey).(bool); ok && v && stockPrompt != "" {
				prompt = stockPrompt
			}
			// 注入用户长期记忆
			if memorySvc != nil {
				memBlock := memorySvc.InjectIntoPrompt(ctx, "default", msg.Content)
				if memBlock != "" {
					prompt += memBlock
				}
			}
			if skills != nil {
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
			// Query rewrite: 口语化/错别字/缩写归一为书面提问
			userContent := msg.Content
			rewritten := rewriteQueryIfNeeded(ctx, chatModel, userContent)
			if rewritten != "" {
				userContent = rewritten
			}

			return []*schema.Message{
				{Role: schema.System, Content: prompt},
				{Role: schema.User, Content: prefixForContent(userContent)},
			}, nil
		},
	))

	graph.AddChatModelNode("chat_model", chatModel)

	graph.AddLambdaNode("parse_tool_calls", compose.InvokableLambda(
		func(ctx context.Context, msg *schema.Message) (*schema.Message, error) {
			if len(msg.ToolCalls) > 0 {
				return msg, nil
			}
			tcs := parsePromptToolCalls(msg.Content)
			if len(tcs) == 0 {
				return msg, nil
			}
			msg.ToolCalls = tcs
			msg.Content = stripToolCallBlocks(msg.Content)
			return msg, nil
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

	// 使用 Eino 内置的 checkpoint 机制
	compileOpts := []compose.GraphCompileOption{compose.WithMaxRunSteps(200)}
	if cpStore != nil {
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
