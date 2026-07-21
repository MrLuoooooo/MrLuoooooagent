package tool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// slowTool 模拟一个可控制延迟的工具。
type slowTool struct {
	delay time.Duration
}

func (s *slowTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "slow_tool", Desc: "test"}, nil
}

func (s *slowTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	select {
	case <-time.After(s.delay):
		return "done", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

var _ Tool = (*slowTool)(nil)

// hangTool 模拟一个永远不返回的工具（模拟罕见bug）。
type hangTool struct{}

func (h *hangTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "hang_tool", Desc: "hangs forever"}, nil
}

func (h *hangTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

var _ Tool = (*hangTool)(nil)

// failTool 模拟一个总是失败的工具。
type failTool struct{}

func (f *failTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "fail_tool", Desc: "always fails"}, nil
}

func (f *failTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	return "", &testError{msg: "模拟工具失败"}
}

var _ Tool = (*failTool)(nil)

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// ——— Tests ———

func TestWrapper_NormalTool(t *testing.T) {
	inner := &slowTool{delay: 10 * time.Millisecond}
	wrapped := WrapWithTimeoutBreaker(inner, WrapperConfig{
		PerToolTimeout: 5 * time.Second,
	})
	ctx := context.Background()
	result, err := wrapped.InvokableRun(ctx, "")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !strings.Contains(result, "done") {
		t.Fatalf("expected 'done', got %q", result)
	}
}

func TestWrapper_Timeout(t *testing.T) {
	inner := &slowTool{delay: 2 * time.Second}
	wrapped := WrapWithTimeoutBreaker(inner, WrapperConfig{
		PerToolTimeout: 50 * time.Millisecond,
		MaxFailures:    10,
	})
	ctx := context.Background()
	_, err := wrapped.InvokableRun(ctx, "")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("expected timeout message, got %v", err)
	}
}

func TestWrapper_CircuitBreaker(t *testing.T) {
	inner := &failTool{}
	wrapped := WrapWithTimeoutBreaker(inner, WrapperConfig{
		MaxFailures:    3,
		BreakerTimeout: 10 * time.Minute,
	})
	ctx := context.Background()

	// 连续失败 3 次触发熔断
	for i := 0; i < 3; i++ {
		_, err := wrapped.InvokableRun(ctx, "")
		if err == nil {
			t.Fatal("expected failure")
		}
	}

	// 第 4 次应该返回熔断错误
	_, err := wrapped.InvokableRun(ctx, "")
	if err == nil {
		t.Fatal("expected breaker open error")
	}
	if !strings.Contains(err.Error(), "熔断") {
		t.Fatalf("expected breaker open message, got %v", err)
	}
}

func TestWrapper_HangTool(t *testing.T) {
	inner := &hangTool{}
	wrapped := WrapWithTimeoutBreaker(inner, WrapperConfig{
		PerToolTimeout: 50 * time.Millisecond,
		MaxFailures:    10,
	})
	ctx := context.Background()
	_, err := wrapped.InvokableRun(ctx, "")
	if err == nil {
		t.Fatal("expected timeout for hang tool")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("expected timeout message, got %v", err)
	}
}

func TestWrapper_InfoPassthrough(t *testing.T) {
	inner := &slowTool{delay: 0}
	wrapped := WrapWithTimeoutBreaker(inner, DefaultWrapperConfig)
	info, err := wrapped.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "slow_tool" {
		t.Fatalf("expected 'slow_tool', got %q", info.Name)
	}
}
