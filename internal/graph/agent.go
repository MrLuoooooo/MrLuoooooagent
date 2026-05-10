package graph

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// NewAgentGraph builds a Runnable[*schema.Message, *schema.Message] that:
//   1. Sends the user message to a ChatModel (with tools bound)
//   2. If the model returns tool calls, executes them via ToolsNode
//   3. Loops the tool results back to the ChatModel
//   4. Exits when the model produces a non-tool-call response
func NewAgentGraph(
	chatModel model.ChatModel,
	toolsNode *compose.ToolsNode,
) (compose.Runnable[*schema.Message, *schema.Message], error) {

	graph := compose.NewGraph[*schema.Message, *schema.Message]()

	// Convert *schema.Message → []*schema.Message for the ChatModel.
	graph.AddLambdaNode("to_messages", compose.InvokableLambda(
		func(ctx context.Context, msg *schema.Message) ([]*schema.Message, error) {
			return []*schema.Message{msg}, nil
		},
	))

	// ChatModel: receives []*schema.Message, returns *schema.Message.
	graph.AddChatModelNode("chat_model", chatModel)

	// Passthrough for ToolsNode output ([]*schema.Message) → ChatModel input ([]*schema.Message).
	graph.AddPassthroughNode("loop_back")

	// ToolsNode: receives *schema.Message, returns []*schema.Message.
	graph.AddToolsNode("tools", toolsNode)

	// ── Edges ───────────────────────────────────────────
	// START → wrap → ChatModel
	graph.AddEdge(compose.START, "to_messages")
	graph.AddEdge("to_messages", "chat_model")

	// ChatModel → branch
	//   - if ToolCalls present → "tools"
	//   - otherwise → END
	graph.AddBranch("chat_model", compose.NewGraphBranch(
		func(ctx context.Context, msg *schema.Message) (string, error) {
			if len(msg.ToolCalls) > 0 {
				return "tools", nil
			}
			return compose.END, nil
		},
		map[string]bool{"tools": true, compose.END: true},
	))

	// tools → loop_back → ChatModel
	graph.AddEdge("tools", "loop_back")
	graph.AddEdge("loop_back", "chat_model")

	return graph.Compile(context.Background())
}

// MustBindTools binds tools to the ChatModel.
// Panics on error — use in initialization only.
func MustBindTools(cm model.ChatModel, tools []*schema.ToolInfo) {
	if err := cm.BindTools(tools); err != nil {
		panic(fmt.Sprintf("bind tools: %v", err))
	}
}
