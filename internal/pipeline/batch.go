package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

// BatchPipeline executes multiple prompts through the agent graph using
// an Eino composed graph for task orchestration, then streams progress via
// a Go channel consumed by the SSE handler.
type BatchPipeline struct {
	agentGraph compose.Runnable[*schema.Message, *schema.Message]
}

// NewBatchPipeline creates a BatchPipeline.
func NewBatchPipeline(agentGraph compose.Runnable[*schema.Message, *schema.Message]) *BatchPipeline {
	return &BatchPipeline{agentGraph: agentGraph}
}

// trySend attempts to send an event to the channel, respecting context cancellation.
// Returns false if the context is done (client disconnected).
func trySend[T any](ctx context.Context, ch chan<- T, v T) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- v:
		return true
	}
}

// Execute runs all tasks sequentially and sends progress events to the returned channel.
// The caller is responsible for reading from the channel until it is closed.
// If ctx is cancelled (client disconnect or timeout), the goroutine exits cleanly.
func (p *BatchPipeline) Execute(
	ctx context.Context,
	tasks []model.BatchTask,
) <-chan model.BatchProgress {
	ch := make(chan model.BatchProgress, 32)

	go func() {
		defer close(ch)

		results := make([]resultEntry, 0, len(tasks))

		for _, task := range tasks {
			// Check context before each task.
			if ctx.Err() != nil {
				trySend(ctx, ch, model.BatchProgress{
					Type: model.BatchSummary,
					Result: fmt.Sprintf("批量任务被取消 (%d 个已完成/%d 个总数)", len(results), len(tasks)),
				})
				return
			}

			if task.ID == "" {
				task.ID = fmt.Sprintf("task_%d", len(results)+1)
			}
			if !trySend(ctx, ch, model.BatchProgress{Type: model.BatchTaskStart, TaskID: task.ID}) {
				return
			}

			userMsg := &schema.Message{Role: schema.User, Content: task.Prompt}
			result, err := p.agentGraph.Invoke(ctx, userMsg)
			if err != nil {
				// If context cancelled, exit cleanly rather than reporting error.
				if ctx.Err() != nil {
					return
				}
				if !trySend(ctx, ch, model.BatchProgress{
					Type: model.BatchTaskError, TaskID: task.ID, Error: err.Error(),
				}) {
					return
				}
				results = append(results, resultEntry{id: task.ID, ok: false, text: err.Error()})
				continue
			}

			if !trySend(ctx, ch, model.BatchProgress{
				Type: model.BatchTaskDone, TaskID: task.ID, Result: result.Content,
			}) {
				return
			}
			results = append(results, resultEntry{id: task.ID, ok: true, text: result.Content})
		}

		// Build a summary.
		var b strings.Builder
		b.WriteString(fmt.Sprintf("批量任务完成 (%d/%d)\n\n", countOK(results), len(results)))
		for _, r := range results {
			status := "✅"
			if !r.ok {
				status = "❌"
			}
			short := r.text
			if len(short) > 120 {
				short = short[:120] + "..."
			}
			b.WriteString(fmt.Sprintf("%s [%s] %s\n", status, r.id, short))
		}

		trySend(ctx, ch, model.BatchProgress{Type: model.BatchSummary, Result: b.String()})
		trySend(ctx, ch, model.BatchProgress{Type: model.BatchDone})
	}()

	return ch
}

type resultEntry struct {
	id   string
	ok   bool
	text string
}

func countOK(results []resultEntry) int {
	n := 0
	for _, r := range results {
		if r.ok {
			n++
		}
	}
	return n
}

// Graph returns an Eino graph that wraps Execute for use within larger composes.
// The input is a []model.BatchTask, output is a single summary string.
func (p *BatchPipeline) Graph() compose.Runnable[[]model.BatchTask, string] {
	g := compose.NewGraph[[]model.BatchTask, string]()

	_ = g.AddLambdaNode("batch_execute", compose.InvokableLambda(
		func(ctx context.Context, tasks []model.BatchTask) (string, error) {
			ch := p.Execute(ctx, tasks)
			var parts []string
			for evt := range ch {
				switch evt.Type {
				case model.BatchTaskDone:
					parts = append(parts, fmt.Sprintf("[%s] %s", evt.TaskID, evt.Result))
				case model.BatchTaskError:
					parts = append(parts, fmt.Sprintf("[%s] ERROR: %s", evt.TaskID, evt.Error))
				}
			}
			return strings.Join(parts, "\n\n"), nil
		},
	))

	_ = g.AddEdge(compose.START, "batch_execute")
	_ = g.AddEdge("batch_execute", compose.END)

	r, _ := g.Compile(context.Background())
	return r
}
