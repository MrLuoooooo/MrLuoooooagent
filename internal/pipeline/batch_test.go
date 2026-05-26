package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

type mockAgent struct {
	reply string
	err   error
	delay time.Duration
	calls int
}

func (m *mockAgent) Invoke(ctx context.Context, msg *schema.Message, opts ...compose.Option) (*schema.Message, error) {
	m.calls++
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return &schema.Message{Role: schema.Assistant, Content: m.reply}, nil
}

func (m *mockAgent) Stream(ctx context.Context, msg *schema.Message, opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *mockAgent) Collect(ctx context.Context, input *schema.StreamReader[*schema.Message], opts ...compose.Option) (*schema.Message, error) {
	return nil, nil
}

func (m *mockAgent) Transform(ctx context.Context, input *schema.StreamReader[*schema.Message], opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func TestBatchExecute_SingleTask(t *testing.T) {
	mock := &mockAgent{reply: "任务完成"}
	bp := NewBatchPipeline(mock)
	ch := bp.Execute(context.Background(), []model.BatchTask{
		{ID: "t1", Prompt: "test"},
	})

	var events []model.BatchProgress
	for evt := range ch {
		events = append(events, evt)
	}

	if events[0].Type != model.BatchTaskStart {
		t.Fatalf("first event = %s, want task_start", events[0].Type)
	}
	if events[1].Type != model.BatchTaskDone {
		t.Fatalf("second event = %s, want task_done", events[1].Type)
	}
	if events[1].Result != "任务完成" {
		t.Errorf("result = %q, want '任务完成'", events[1].Result)
	}
	lastType := events[len(events)-1].Type
	if lastType != model.BatchDone {
		t.Errorf("last event = %s, want done", lastType)
	}
}

func TestBatchExecute_TaskError(t *testing.T) {
	mock := &mockAgent{err: context.DeadlineExceeded}
	bp := NewBatchPipeline(mock)
	ch := bp.Execute(context.Background(), []model.BatchTask{
		{ID: "t1", Prompt: "will fail"},
	})

	var events []model.BatchProgress
	for evt := range ch {
		events = append(events, evt)
	}

	found := false
	for _, evt := range events {
		if evt.Type == model.BatchTaskError {
			found = true
			if evt.TaskID != "t1" {
				t.Errorf("error task id = %q", evt.TaskID)
			}
		}
	}
	if !found {
		t.Fatal("expected task_error event")
	}
}

func TestBatchExecute_MultipleTasks(t *testing.T) {
	mock := &mockAgent{reply: "done"}
	bp := NewBatchPipeline(mock)
	ch := bp.Execute(context.Background(), []model.BatchTask{
		{ID: "a", Prompt: "p1"},
		{ID: "b", Prompt: "p2"},
		{ID: "c", Prompt: "p3"},
	})

	var starts, dones int
	for evt := range ch {
		switch evt.Type {
		case model.BatchTaskStart:
			starts++
		case model.BatchTaskDone:
			dones++
		}
	}
	if starts != 3 {
		t.Errorf("starts = %d, want 3", starts)
	}
	if dones != 3 {
		t.Errorf("dones = %d, want 3", dones)
	}
	if mock.calls != 3 {
		t.Errorf("calls = %d, want 3", mock.calls)
	}
}

func TestBatchExecute_AutoGenerateID(t *testing.T) {
	mock := &mockAgent{reply: "ok"}
	bp := NewBatchPipeline(mock)
	ch := bp.Execute(context.Background(), []model.BatchTask{
		{Prompt: "no id"},
	})

	var events []model.BatchProgress
	for evt := range ch {
		events = append(events, evt)
	}

	if events[0].TaskID != "task_1" {
		t.Errorf("auto id = %q, want task_1", events[0].TaskID)
	}
}

func TestBatchExecute_ContextCancellation(t *testing.T) {
	mock := &mockAgent{reply: "slow", delay: 2 * time.Second}
	bp := NewBatchPipeline(mock)

	ctx, cancel := context.WithCancel(context.Background())
	ch := bp.Execute(ctx, []model.BatchTask{
		{ID: "t1", Prompt: "task1"},
		{ID: "t2", Prompt: "task2"},
	})

	// Read the first task_start, then cancel.
	evt := <-ch
	if evt.Type != model.BatchTaskStart {
		t.Fatalf("expected task_start, got %s", evt.Type)
	}
	cancel()

	// The goroutine should exit cleanly; channel will close.
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // channel closed, goroutine exited cleanly
			}
		case <-timeout:
			t.Fatal("timeout: goroutine did not exit after context cancel")
		}
	}
}

func TestBatchExecute_EmptyTasks(t *testing.T) {
	mock := &mockAgent{reply: "unused"}
	bp := NewBatchPipeline(mock)
	ch := bp.Execute(context.Background(), []model.BatchTask{})

	var events []model.BatchProgress
	for evt := range ch {
		events = append(events, evt)
	}
	// Should get summary + done only.
	if events[0].Type != model.BatchSummary {
		t.Errorf("first event = %s, want summary", events[0].Type)
	}
	if events[1].Type != model.BatchDone {
		t.Errorf("second event = %s, want done", events[1].Type)
	}
}

func TestBatchPipeline_Graph(t *testing.T) {
	mock := &mockAgent{reply: "graph output"}
	bp := NewBatchPipeline(mock)
	r := bp.Graph()

	result, err := r.Invoke(context.Background(), []model.BatchTask{
		{ID: "g1", Prompt: "graph prompt"},
	})
	if err != nil {
		t.Fatalf("graph invoke: %v", err)
	}
	if result == "" {
		t.Fatal("graph returned empty result")
	}
}
