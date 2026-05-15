package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// RAGTool wraps the RAG pipeline as a Tool so the Agent can invoke it.
// This makes the Agent a "meta-agent" that can trigger retrieval on demand.
type RAGTool struct {
	ragFn func(ctx context.Context, query string) (string, error)
}

// NewRAGTool creates a RAG tool backed by the given query function.
// The function should execute the full RAG chain (retrieve → generate) and return the answer.
func NewRAGTool(ragFn func(ctx context.Context, query string) (string, error)) *RAGTool {
	return &RAGTool{ragFn: ragFn}
}

// Info implements tool.BaseTool.
func (r *RAGTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "retrieve_knowledge",
		Desc: "基于用户问题检索知识库并生成回答。当需要查询文档、知识库或历史对话内容时使用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "用户的原始问题",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun implements tool.InvokableTool.
func (r *RAGTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("rag tool: invalid args: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("rag tool: query is required")
	}
	answer, err := r.ragFn(ctx, args.Query)
	if err != nil {
		return "", fmt.Errorf("rag tool: %w", err)
	}
	return answer, nil
}

var _ tool.InvokableTool = (*RAGTool)(nil)
