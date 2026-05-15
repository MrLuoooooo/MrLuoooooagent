package graph

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// NewAgentGraph builds a Runnable[*schema.Message, *schema.Message] that:
//  1. Sends the user message to a ChatModel (with tools bound)
//  2. If the model returns tool calls (native or prompt-parsed), executes them via ToolsNode
//  3. Loops the tool results back to the ChatModel
//  4. Exits when the model produces a non-tool-call response
func NewAgentGraph(
	chatModel model.ChatModel,
	toolsNode *compose.ToolsNode,
	toolInfos []*schema.ToolInfo,
) (compose.Runnable[*schema.Message, *schema.Message], error) {

	// Build dynamic system prompt that includes tool descriptions.
	// This supports both native function calling AND prompt-based tool calling
	// as a fallback for models that don't support native function calling.
	sysPrompt := baseSystemPrompt + buildPromptToolsSection(toolInfos)

	graph := compose.NewGraph[*schema.Message, *schema.Message]()

	graph.AddLambdaNode("to_messages", compose.InvokableLambda(
		func(ctx context.Context, msg *schema.Message) ([]*schema.Message, error) {
			return []*schema.Message{
				{Role: schema.System, Content: sysPrompt},
				msg,
			}, nil
		},
	))

	// ChatModel: receives []*schema.Message, returns *schema.Message.
	graph.AddChatModelNode("chat_model", chatModel)

	// parse_tool_calls: if the model didn't produce native ToolCalls,
	// try to parse prompt-format tool calls from the text content.
	graph.AddLambdaNode("parse_tool_calls", compose.InvokableLambda(
		func(ctx context.Context, msg *schema.Message) (*schema.Message, error) {
			if len(msg.ToolCalls) > 0 {
				// Already has native tool calls — pass through unchanged.
				return msg, nil
			}
			tcs := parsePromptToolCalls(msg.Content)
			if len(tcs) == 0 {
				return msg, nil
			}
			// Found prompt-format tool calls.
			msg.ToolCalls = tcs
			// Strip tool call blocks from visible content.
			msg.Content = stripToolCallBlocks(msg.Content)
			return msg, nil
		},
	))

	// ToolsNode: receives *schema.Message, returns []*schema.Message.
	graph.AddToolsNode("tools", toolsNode)

	// ── Edges ───────────────────────────────────────────
	graph.AddEdge(compose.START, "to_messages")
	graph.AddEdge("to_messages", "chat_model")
	graph.AddEdge("chat_model", "parse_tool_calls")

	// parse_tool_calls → branch
	//   - if ToolCalls present → "tools"
	//   - otherwise → END
	graph.AddBranch("parse_tool_calls", compose.NewGraphBranch(
		func(ctx context.Context, msg *schema.Message) (string, error) {
			if len(msg.ToolCalls) > 0 {
				return "tools", nil
			}
			return compose.END, nil
		},
		map[string]bool{"tools": true, compose.END: true},
	))

	// tools → ChatModel (direct feedback loop)
	graph.AddEdge("tools", "chat_model")

	return graph.Compile(context.Background())
}

// MustBindTools binds tools to the ChatModel.
// Panics on error — use in initialization only.
func MustBindTools(cm model.ChatModel, tools []*schema.ToolInfo) {
	if err := cm.BindTools(tools); err != nil {
		panic(fmt.Sprintf("bind tools: %v", err))
	}
}

// baseSystemPrompt tells the LLM about its capabilities.
// Tool descriptions are appended dynamically at graph construction time.
const baseSystemPrompt = `你是一个具有本地文件系统和命令行访问能力的 AI 助手，运行在 Windows 系统上（项目根目录 D:\goagentpro）。

## 重要规则
1. 所有文件路径默认相对于 D:\goagentpro，你不需要拼接 D:\goagentpro 前缀
2. 使用 edit_file 修改代码时，old_string 必须精确唯一匹配
3. 执行命令前先考虑安全性，高危命令会被自动拦截
4. 读取大文件时会自动截断，可以用 max_size 控制
5. 主动使用工具完成任务，不要说你做不了——你拥有这些能力
6. 当用户询问需要物理访问的问题（如"查看 D 盘文件夹"），直接使用 list_directory 或 execute_command
7. 不要猜测或假设你无法做到的事情，先用工具尝试

## 工具调用格式
当需要使用工具时，在回复中输出：

<tool_call>
{"name": "工具名", "arguments": {"参数名": "参数值"}}
</tool_call>

可以同时调用多个工具。工具结果会自动返回。不需要工具时直接文字回复。
`
