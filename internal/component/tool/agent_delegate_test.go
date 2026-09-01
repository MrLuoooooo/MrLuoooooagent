package tool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// makeRunnable 把 lambda 编译成最小单节点图，得到 compose.Runnable。
func makeRunnable(fn func(ctx context.Context, msg *schema.Message) (*schema.Message, error)) compose.Runnable[*schema.Message, *schema.Message] {
	g := compose.NewGraph[*schema.Message, *schema.Message]()
	g.AddLambdaNode("node", compose.InvokableLambda(fn))
	g.AddEdge(compose.START, "node")
	g.AddEdge("node", compose.END)
	r, err := g.Compile(context.Background())
	if err != nil {
		panic(err)
	}
	return r
}

func fakeAgent(content string) compose.Runnable[*schema.Message, *schema.Message] {
	return makeRunnable(func(ctx context.Context, msg *schema.Message) (*schema.Message, error) {
		return &schema.Message{Role: schema.Assistant, Content: content}, nil
	})
}

func newTestDelegate(t *testing.T, timeout time.Duration, agents map[string]compose.Runnable[*schema.Message, *schema.Message]) *AgentDelegateTool {
	t.Helper()
	if timeout <= 0 {
		timeout = time.Second
	}
	return NewAgentDelegateTool(agents, timeout, zap.NewNop())
}

func TestAgentDelegateTool_Info(t *testing.T) {
	dt := newTestDelegate(t, 0, map[string]compose.Runnable[*schema.Message, *schema.Message]{
		"stock_analyst": fakeAgent("x"),
	})
	info, err := dt.Info(context.Background())
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Name != "delegate_to_agent" {
		t.Fatalf("name want delegate_to_agent, got %s", info.Name)
	}
	if !strings.Contains(info.Desc, "stock_analyst") {
		t.Fatalf("desc should list available agents: %s", info.Desc)
	}
}

func TestAgentDelegateTool_InvokeOK(t *testing.T) {
	dt := newTestDelegate(t, 0, map[string]compose.Runnable[*schema.Message, *schema.Message]{
		"stock_analyst": fakeAgent("茅台现价 1255.67"),
	})
	out, err := dt.InvokableRun(context.Background(), `{"agent":"stock_analyst","query":"分析茅台"}`)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if !strings.Contains(out, "1255.67") {
		t.Fatalf("want fake answer, got %q", out)
	}
}

func TestAgentDelegateTool_UnknownAgent(t *testing.T) {
	dt := newTestDelegate(t, 0, map[string]compose.Runnable[*schema.Message, *schema.Message]{
		"stock_analyst": fakeAgent("x"),
	})
	_, err := dt.InvokableRun(context.Background(), `{"agent":"nonexistent","query":"hi"}`)
	if err == nil || !strings.Contains(err.Error(), "unknown sub agent") {
		t.Fatalf("want unknown agent error, got %v", err)
	}
}

func TestAgentDelegateTool_MissingArgs(t *testing.T) {
	dt := newTestDelegate(t, 0, map[string]compose.Runnable[*schema.Message, *schema.Message]{
		"stock_analyst": fakeAgent("x"),
	})
	if _, err := dt.InvokableRun(context.Background(), `{"agent":""}`); err == nil {
		t.Fatal("want error for empty agent")
	}
	if _, err := dt.InvokableRun(context.Background(), `not-json`); err == nil {
		t.Fatal("want error for invalid json")
	}
}

func TestAgentDelegateTool_Timeout(t *testing.T) {
	slow := makeRunnable(func(ctx context.Context, msg *schema.Message) (*schema.Message, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
			return &schema.Message{Content: "late"}, nil
		}
	})
	dt := newTestDelegate(t, 100*time.Millisecond, map[string]compose.Runnable[*schema.Message, *schema.Message]{
		"slow": slow,
	})
	_, err := dt.InvokableRun(context.Background(), `{"agent":"slow","query":"hi"}`)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("want timeout error, got %v", err)
	}
}

func TestAgentDelegateTool_SubAgentError(t *testing.T) {
	failing := makeRunnable(func(ctx context.Context, msg *schema.Message) (*schema.Message, error) {
		return nil, context.Canceled
	})
	dt := newTestDelegate(t, 0, map[string]compose.Runnable[*schema.Message, *schema.Message]{
		"bad": failing,
	})
	_, err := dt.InvokableRun(context.Background(), `{"agent":"bad","query":"hi"}`)
	if err == nil {
		t.Fatal("want propagated error")
	}
}

func TestAgentDelegateTool_DefaultTimeout(t *testing.T) {
	dt := NewAgentDelegateTool(map[string]compose.Runnable[*schema.Message, *schema.Message]{}, 0, zap.NewNop())
	if dt.timeout != 90*time.Second {
		t.Fatalf("default timeout want 90s, got %s", dt.timeout)
	}
}
