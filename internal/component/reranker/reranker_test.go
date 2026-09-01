package reranker

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeChatModel 根据文档内容返回分数（"score:8"），并记录最大并发。
type fakeChatModel struct {
	inflight    atomic.Int64
	maxInflight atomic.Int64
}

func (f *fakeChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	cur := f.inflight.Add(1)
	defer f.inflight.Add(-1)
	for {
		m := f.maxInflight.Load()
		if cur <= m || f.maxInflight.CompareAndSwap(m, cur) {
			break
		}
	}
	time.Sleep(10 * time.Millisecond) // 模拟 LLM 延迟，放大并发窗口
	doc := input[len(input)-1].Content
	var score string
	switch {
	case strings.Contains(doc, "score:9"):
		score = "9"
	case strings.Contains(doc, "score:5"):
		score = "5"
	default:
		score = "1"
	}
	return &schema.Message{Role: schema.Assistant, Content: score}, nil
}

func (f *fakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (f *fakeChatModel) BindTools(tools []*schema.ToolInfo) error { return nil }
func (f *fakeChatModel) GetType() string                          { return "fake" }

func TestRerank_Concurrent(t *testing.T) {
	cm := &fakeChatModel{}
	r := NewLLMReranker(cm).WithMaxConcurrency(4)
	ctx := context.Background()

	docs := []*schema.Document{
		{Content: "doc with score:5"},
		{Content: "doc with score:9"},
		{Content: "plain doc"},
		{Content: "doc with score:5"},
		{Content: "another score:9"},
	}
	start := time.Now()
	got, err := r.Rerank(ctx, "query", docs, 3)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("rerank: %v", err)
	}
	// 并发验证：5 篇各 10ms，串行需 ~50ms，4 并发应显著更快
	if elapsed > 35*time.Millisecond {
		t.Fatalf("expected concurrency speedup, took %v", elapsed)
	}
	// 排序验证：score:9 的应在最前
	if !strings.Contains(got[0].Content, "score:9") {
		t.Fatalf("top doc should be score:9, got %q", got[0].Content)
	}
	if len(got) != 3 {
		t.Fatalf("topN want 3, got %d", len(got))
	}
}

func TestRerank_ConcurrencyCap(t *testing.T) {
	cm := &fakeChatModel{}
	r := NewLLMReranker(cm).WithMaxConcurrency(2)
	ctx := context.Background()

	docs := make([]*schema.Document, 8)
	for i := range docs {
		docs[i] = &schema.Document{Content: "doc"}
	}
	if _, err := r.Rerank(ctx, "query", docs, 3); err != nil {
		t.Fatalf("rerank: %v", err)
	}
	if cm.maxInflight.Load() > 2 {
		t.Fatalf("concurrency cap violated: max %d > 2", cm.maxInflight.Load())
	}
}

func TestRerank_SingleFailureDegrades(t *testing.T) {
	// 模型返回无法解析的内容时全部降级 0 分，不报错，返回前 topN 篇
	bad := &badModel{}
	r := NewLLMReranker(bad)
	docs := []*schema.Document{{Content: "a"}, {Content: "b"}, {Content: "c"}}
	got, err := r.Rerank(context.Background(), "q", docs, 2)
	if err != nil {
		t.Fatalf("should not fail on unparseable scores: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 docs, got %d", len(got))
	}
}

type badModel struct{}

func (badModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "not a number at all"}, nil
}
func (badModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}
func (badModel) BindTools(tools []*schema.ToolInfo) error { return nil }
func (badModel) GetType() string                          { return "bad" }

// panicModel 模拟上游模型库异常：Generate 直接 panic。
type panicModel struct{}

func (panicModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	panic("upstream model exploded")
}
func (panicModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}
func (panicModel) BindTools(tools []*schema.ToolInfo) error { return nil }
func (panicModel) GetType() string                          { return "panic" }

// TestRerank_ModelPanicDoesNotCrash goroutine panic 必须被 recover，
// 否则整个 agent 进程崩溃（goroutine panic 外层接不住）。
func TestRerank_ModelPanicDoesNotCrash(t *testing.T) {
	r := NewLLMReranker(panicModel{})
	docs := []*schema.Document{{Content: "a"}, {Content: "b"}, {Content: "c"}}
	got, err := r.Rerank(context.Background(), "q", docs, 2)
	if err != nil {
		t.Fatalf("should not error on panic: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 docs (degraded to 0 scores), got %d", len(got))
	}
}
