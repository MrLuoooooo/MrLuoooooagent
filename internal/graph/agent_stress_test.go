package graph

import (
	"context"
	"fmt"
	"sync"
	"testing"

	eino_tool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// TestStress_MultipleTools 压测并发工具调用场景。
func TestStress_MultipleTools(t *testing.T) {
	cm := &toolCallingModel{
		responses: []*schema.Message{
			{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{
					toolCall("get_price", `{"symbol":"600519"}`).ToolCalls[0],
					toolCall("get_revenue", `{"symbol":"600519","year":"2025"}`).ToolCalls[0],
					toolCall("get_growth", `{"symbol":"600519","year":"2025"}`).ToolCalls[0],
				},
			},
			{Role: schema.Assistant, Content: "分析完成"},
		},
	}

	simpleTool := func(name string) *mockTool {
		return &mockTool{name: name, desc: name, handler: func(args string) string { return name + " result" }}
	}

	tn, _ := compose.NewToolNode(context.Background(), &compose.ToolsNodeConfig{
		Tools: []eino_tool.BaseTool{
			simpleTool("get_price"),
			simpleTool("get_revenue"),
			simpleTool("get_growth"),
		},
	})

	g, _ := NewAgentGraph(cm, tn, []*schema.ToolInfo{
		{Name: "get_price", Desc: "price"},
		{Name: "get_revenue", Desc: "revenue"},
		{Name: "get_growth", Desc: "growth"},
	}, nil, nil, nil, "", "", nil, NewRetryGate(3))

	// 模拟 50 个并发用户同时调 Agent
	concurrency := 50
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := g.Invoke(context.Background(), &schema.Message{
				Role: schema.User, Content: "analyze 600519",
			})
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", id, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	failCount := 0
	for range errs {
		failCount++
	}
	if failCount > 0 {
		t.Fatalf("%d/%d goroutines failed under load", failCount, concurrency)
	}
	t.Logf("50 并发 Agent 全链路通过，无 panic/无阻塞")
}
