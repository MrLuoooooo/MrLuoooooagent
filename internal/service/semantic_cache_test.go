package service

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

// fakeEmbedder 按文本查表返回固定向量，用于可控的相似度测试。
type fakeEmbedder struct {
	vecs map[string][]float64
}

func (f *fakeEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, t := range texts {
		if v, ok := f.vecs[t]; ok {
			out[i] = v
		} else {
			out[i] = []float64{0, 0}
		}
	}
	return out, nil
}

func newTestCache(enabled bool) *SemanticCache {
	emb := &fakeEmbedder{vecs: map[string][]float64{
		"分析茅台":     {1, 0},
		"茅台怎么样":    {0.99, 0.01}, // 与"分析茅台"高相似
		"写首诗":      {0, 1},      // 与"分析茅台"正交
		"茅台基本面":    {0.98, 0.05},
	}}
	return NewSemanticCache(emb, enabled, 0.92, 4, time.Minute)
}

func TestSemanticCache_ExactAndSimilarHit(t *testing.T) {
	c := newTestCache(true)
	ctx := context.Background()

	c.Put(ctx, "分析茅台", "茅台是白酒龙头...")

	// 精确命中
	ans, hit := c.Get(ctx, "分析茅台")
	if !hit || ans != "茅台是白酒龙头..." {
		t.Fatalf("exact hit failed: hit=%v ans=%q", hit, ans)
	}
	// 相似问法命中（语义缓存核心价值）
	if _, hit := c.Get(ctx, "茅台怎么样"); !hit {
		t.Fatal("similar question should hit")
	}
	// 无关问题不命中
	if _, hit := c.Get(ctx, "写首诗"); hit {
		t.Fatal("unrelated question should miss")
	}
}

func TestSemanticCache_Disabled(t *testing.T) {
	c := newTestCache(false)
	ctx := context.Background()
	c.Put(ctx, "分析茅台", "x")
	if _, hit := c.Get(ctx, "分析茅台"); hit {
		t.Fatal("disabled cache must miss")
	}
}

func TestSemanticCache_RealtimeQuerySkipped(t *testing.T) {
	c := newTestCache(true)
	ctx := context.Background()
	c.Put(ctx, "分析茅台", "x")
	// 含股票代码
	if _, hit := c.Get(ctx, "sh600519 现在多少"); hit {
		t.Fatal("stock code query must skip cache")
	}
	// 含时效词
	if _, hit := c.Get(ctx, "今天大盘怎么样"); hit {
		t.Fatal("realtime word query must skip cache")
	}
	// 实时 query 不写入
	c.Put(ctx, "sh600519 涨了吗", "y")
	if _, hit := c.Get(ctx, "sh600519 涨了吗"); hit {
		t.Fatal("realtime query must not be cached")
	}
}

func TestSemanticCache_TTLExpiry(t *testing.T) {
	c := NewSemanticCache(&fakeEmbedder{vecs: map[string][]float64{"q": {1, 0}}}, true, 0.92, 4, 50*time.Millisecond)
	ctx := context.Background()
	c.Put(ctx, "q", "ans")
	if _, hit := c.Get(ctx, "q"); !hit {
		t.Fatal("should hit before TTL")
	}
	time.Sleep(80 * time.Millisecond)
	if _, hit := c.Get(ctx, "q"); hit {
		t.Fatal("should miss after TTL")
	}
}

func TestSemanticCache_LRUEviction(t *testing.T) {
	emb := &fakeEmbedder{}
	c := NewSemanticCache(emb, true, 0.92, 2, time.Minute) // 容量 2
	ctx := context.Background()
	// 用不同向量区分条目
	emb.vecs = map[string][]float64{
		"a": {1, 0, 0, 0},
		"b": {0, 1, 0, 0},
		"c": {0, 0, 1, 0},
	}
	c.Put(ctx, "a", "A")
	c.Put(ctx, "b", "B")
	c.Get(ctx, "a") // a 变最近使用
	c.Put(ctx, "c", "C") // 淘汰最久未用 = b
	if _, hit := c.Get(ctx, "a"); !hit {
		t.Fatal("a should survive (recently used)")
	}
	if _, hit := c.Get(ctx, "b"); hit {
		t.Fatal("b should be evicted (LRU)")
	}
	if _, hit := c.Get(ctx, "c"); !hit {
		t.Fatal("c should be present")
	}
}

func TestSemanticCache_Stats(t *testing.T) {
	c := newTestCache(true)
	ctx := context.Background()
	c.Put(ctx, "分析茅台", "x")
	c.Get(ctx, "分析茅台") // hit
	c.Get(ctx, "写首诗")   // miss
	hits, misses := c.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("stats want 1/1, got %d/%d", hits, misses)
	}
}

func TestCosine(t *testing.T) {
	if v := cosine([]float64{1, 0}, []float64{1, 0}); v < 0.999 {
		t.Fatalf("parallel vectors cos should be ~1, got %v", v)
	}
	if v := cosine([]float64{1, 0}, []float64{0, 1}); v > 1e-9 {
		t.Fatalf("orthogonal vectors cos should be 0, got %v", v)
	}
	if v := cosine([]float64{1, 0}, []float64{1, 0, 1}); v != 0 {
		t.Fatalf("dim mismatch cos should be 0, got %v", v)
	}
	if v := cosine(nil, nil); v != 0 {
		t.Fatalf("empty vectors cos should be 0, got %v", v)
	}
}

func TestIsRealtimeQuery(t *testing.T) {
	realtime := []string{"sh600519 现在多少", "sz000001 现价", "600519.SH 涨跌", "最新行情", "今天大盘"}
	for _, q := range realtime {
		if !isRealtimeQuery(q) {
			t.Fatalf("%q should be realtime", q)
		}
	}
	normal := []string{"什么是 RAG", "分析茅台的财务", "帮我写一段 Go 代码"}
	for _, q := range normal {
		if isRealtimeQuery(q) {
			t.Fatalf("%q should NOT be realtime", q)
		}
	}
}
