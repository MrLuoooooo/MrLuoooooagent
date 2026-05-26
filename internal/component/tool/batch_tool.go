package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/pipeline"
)

// BatchTool wraps the BatchPipeline as a Tool for the Agent.
// The pipeline is set lazily after graph compilation to break the DI cycle.
type BatchTool struct {
	bp atomic.Value // stores *pipeline.BatchPipeline
}

// batchToolInstance is the singleton registered in the tool registry.
var batchToolInstance = &BatchTool{}

// NewBatchTool returns the singleton BatchTool. Register once, set pipeline later.
func NewBatchTool() *BatchTool { return batchToolInstance }

// SetBatchPipeline sets the backing BatchPipeline on the singleton.
// Called after DI constructs the pipeline (post-graph-compile).
func SetBatchPipeline(bp *pipeline.BatchPipeline) { batchToolInstance.bp.Store(bp) }

func (t *BatchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "run_batch",
		Desc: "批量执行多个子任务并返回汇总结果。当用户要求同时处理多项工作时使用，如分析多个文件、查询多只股票。tasks 参数为JSON数组：[{\"id\":\"任务标识\",\"prompt\":\"任务描述\"}]，最多10个任务。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"tasks": {
				Type: schema.String, Desc: "JSON数组格式的任务列表", Required: true,
			},
		}),
	}, nil
}

func (t *BatchTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	bp := t.bp.Load()
	pipeline, ok := bp.(*pipeline.BatchPipeline)
	if !ok {
		return "", fmt.Errorf("run_batch: batch pipeline not initialized yet")
	}

	var tasks []model.BatchTask

	// Try array format: {"tasks": [{"id":"1","prompt":"..."}]}
	var arr struct{ Tasks []model.BatchTask `json:"tasks"` }
	if err := json.Unmarshal([]byte(argsJSON), &arr); err == nil && len(arr.Tasks) > 0 {
		tasks = arr.Tasks
	} else {
		// Try string format: {"tasks": "id1:prompt1\nid2:prompt2"}
		var str struct{ Tasks string `json:"tasks"` }
		if err := json.Unmarshal([]byte(argsJSON), &str); err != nil {
			return "", fmt.Errorf("run_batch: 参数格式错误，应为JSON数组 [{\"id\":\"1\",\"prompt\":\"...\"}]")
		}
		if str.Tasks == "" {
			return "", fmt.Errorf("run_batch: tasks 不能为空")
		}
		// Auto-generate batch tasks from string.
		for i, line := range strings.Split(str.Tasks, "\n") {
			line = strings.TrimSpace(line)
			if line == "" { continue }
			tasks = append(tasks, model.BatchTask{
				ID:     fmt.Sprintf("task_%d", i+1),
				Prompt: line,
			})
		}
	}

	if len(tasks) == 0 {
		return "", fmt.Errorf("run_batch: tasks cannot be empty")
	}
	if len(tasks) > 10 {
		return "", fmt.Errorf("run_batch: max 10 tasks, got %d", len(tasks))
	}

	ch := pipeline.Execute(ctx, tasks)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("批量任务开始 (%d 个)\n", len(tasks)))
	for evt := range ch {
		switch evt.Type {
		case model.BatchTaskStart:
			b.WriteString(fmt.Sprintf("  ▶ [%s] 执行中...\n", evt.TaskID))
		case model.BatchTaskDone:
			s := evt.Result
			if len(s) > 200 { s = s[:200] + "..." }
			b.WriteString(fmt.Sprintf("  ✅ [%s] %s\n", evt.TaskID, s))
		case model.BatchTaskError:
			b.WriteString(fmt.Sprintf("  ❌ [%s] %s\n", evt.TaskID, evt.Error))
		case model.BatchSummary:
			b.WriteString("\n" + evt.Result)
		}
	}
	return b.String(), nil
}

var _ tool.InvokableTool = (*BatchTool)(nil)
