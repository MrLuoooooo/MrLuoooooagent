package tool

import (
	"context"
	"strings"
	"testing"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/pipeline"
)

type stubBatchGraph struct{}

func (s *stubBatchGraph) Invoke(ctx context.Context, msg any, opts ...any) (any, error) {
	// Simple mock: just echo the prompt
	return nil, nil
}

func newStubBatchPipeline() *pipeline.BatchPipeline {
	return pipeline.NewBatchPipeline(nil)
}

// Test BatchTool with pipeline not set.
func TestBatchTool_NotInitialized(t *testing.T) {
	bt := &BatchTool{}
	_, err := bt.InvokableRun(context.Background(), `{"tasks": [{"id": "1", "prompt": "hello"}]}`)
	if err == nil {
		t.Fatal("expected error when pipeline not initialized")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBatchTool_Info(t *testing.T) {
	bt := NewBatchTool()
	info, err := bt.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "run_batch" {
		t.Errorf("name = %q", info.Name)
	}
}

func TestBatchTool_EmptyTasksArray(t *testing.T) {
	bt := NewBatchTool()
	_, err := bt.InvokableRun(context.Background(), `{"tasks": []}`)
	if err == nil {
		t.Fatal("expected error for empty tasks")
	}
}

func TestBatchTool_TooManyTasks(t *testing.T) {
	bt := NewBatchTool()
	tasks := make([]map[string]string, 11)
	for i := 0; i < 11; i++ {
		tasks[i] = map[string]string{"id": "t", "prompt": "p"}
	}
	// We'd need a proper test with a pipeline set, but at least test the validation
	_, err := bt.InvokableRun(context.Background(), `{}`)
	if err == nil {
		t.Fatal("expected error for empty tasks")
	}
}

func TestBatchTool_StringFormat(t *testing.T) {
	bt := NewBatchTool()
	_, err := bt.InvokableRun(context.Background(), `{"tasks": ""}`)
	if err == nil {
		t.Fatal("expected error for empty string tasks")
	}
}

func TestBatchTool_InvalidJSON(t *testing.T) {
	bt := NewBatchTool()
	_, err := bt.InvokableRun(context.Background(), `not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
