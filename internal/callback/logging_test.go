package callback

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/callbacks"
	"go.uber.org/zap"
)

func TestNewLoggingCallback_Builds(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handler := NewLoggingCallback(logger)

	if handler == nil {
		t.Fatal("NewLoggingCallback() returned nil")
	}

	// Verify it satisfies the Handler interface.
	var _ callbacks.Handler = handler
}

func TestLoggingCallback_OnStartEnd(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handler := NewLoggingCallback(logger)

	ctx := context.Background()
	runInfo := &callbacks.RunInfo{
		Name:      "test_node",
		Type:      "test",
		Component: "test",
	}

	// OnStart should return a context with start_time.
	ctx = handler.OnStart(ctx, runInfo, "test input")
	if ctx == nil {
		t.Error("OnStart returned nil context")
	}

	// OnEnd should not panic.
	ctx = handler.OnEnd(ctx, runInfo, "test output")
	if ctx == nil {
		t.Error("OnEnd returned nil context")
	}
}

func TestLoggingCallback_OnError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	handler := NewLoggingCallback(logger)

	ctx := context.Background()
	runInfo := &callbacks.RunInfo{
		Name: "error_node",
		Type: "openai",
	}

	ctx = handler.OnError(ctx, runInfo, context.DeadlineExceeded)
	if ctx == nil {
		t.Error("OnError returned nil context")
	}
}
