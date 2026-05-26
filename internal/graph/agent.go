package graph

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/tool"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
)

func NewAgentGraph(
	chatModel model.ChatModel,
	toolsNode *compose.ToolsNode,
	toolInfos []*schema.ToolInfo,
	skills *service.SkillStore,
	systemPrompt string,
) (compose.Runnable[*schema.Message, *schema.Message], error) {

	sysPrompt := systemPrompt

	graph := compose.NewGraph[*schema.Message, *schema.Message]()

	graph.AddLambdaNode("to_messages", compose.InvokableLambda(
		func(ctx context.Context, msg *schema.Message) ([]*schema.Message, error) {
			prompt := sysPrompt
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
			// Inject current workspace.
			win := tool.GetWorkspaceWinPath()
			if win == "" {
				win = tool.GetWorkspaceRoot()
			}
			if win != "" {
				summary := tool.ReadWorkspaceSummary()
				prompt += "\n## 当前工作目录: " + win + "\n目录内容：" + summary
			}
			return []*schema.Message{
				{Role: schema.System, Content: prompt},
				{Role: schema.User, Content: prefixForContent(msg.Content)},
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

	graph.AddEdge("tools", "tool_as_user")
	graph.AddEdge("tool_as_user", "chat_model")

	return graph.Compile(context.Background(), compose.WithMaxRunSteps(200))
}

// prefixForContent detects write/create intent and guides tool usage.
func prefixForContent(content string) string {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "写") || strings.Contains(lower, "创建") || strings.Contains(lower, "write") || strings.Contains(lower, "create") {
		return "TASK: " + content + " (Use write_file tool now. Do not list files or describe workspace.)"
	}
	return content
}
