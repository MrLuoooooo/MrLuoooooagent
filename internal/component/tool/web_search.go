package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// WebSearchTool performs web searches.
// Currently a placeholder — returns a descriptive message until a search backend is wired.
type WebSearchTool struct{}

// Info implements tool.BaseTool.
func (w *WebSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "web_search",
		Desc: "搜索互联网获取最新信息。当需要实时数据、新闻、或不在知识库中的内容时使用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "搜索关键词",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun implements tool.InvokableTool.
func (w *WebSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("web_search: invalid args: %w", err)
	}
	return fmt.Sprintf("[web_search] 搜索结果占位: %q（需要接入搜索 API 后生效）", args.Query), nil
}
