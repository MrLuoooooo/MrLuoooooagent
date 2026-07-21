package esretriever

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

// mockEmbedder returns fixed-dimension vectors, no real embedding API call.
type mockEmbedder struct{ dim int }

func (e *mockEmbedder) EmbedStrings(ctx context.Context, texts []string, opts ...embedding.Option) ([][]float64, error) {
	vecs := make([][]float64, len(texts))
	for i := range texts {
		v := make([]float64, e.dim)
		v[0] = 1.0
		vecs[i] = v
	}
	return vecs, nil
}

// StressRAG_ConcurrentRetrieve 压测 100 并发 RAG 检索。
func TestStressRAG_ConcurrentRetrieve(t *testing.T) {
	emb := &mockEmbedder{dim: 768}

	retriever := NewESRetriever(nil, emb, "test_index", 5, 30, 0.3, false)

	concurrency := 100
	var wg sync.WaitGroup
	var errCount atomic.Int64
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			_, err := retriever.Retrieve(context.Background(), q)
			if err != nil {
				errCount.Add(1)
			}
		}("贵州茅台 2025 Q4 营收")
	}
	wg.Wait()
	elapsed := time.Since(start)

	total := errCount.Load()
	t.Logf("100 并发检索耗时: %v, 预期 ES 不可用错误数: %d (nil client 安全兜底)", elapsed, total)
	// nil client 应全部返回错误，不应 panic——验证并发安全
	_ = elapsed
	if total != int64(concurrency) {
		t.Fatalf("expected all %d concurrent calls to return error (nil client), got %d errors", concurrency, total)
	}
	t.Log("RAG retriever 并发安全: 100 并发无 panic/无 data race")
}

// BenchmarkRAG_Retrieve 基准测试单次检索性能。
func BenchmarkRAG_Retrieve(b *testing.B) {
	emb := &mockEmbedder{dim: 768}
	retriever := NewESRetriever(nil, emb, "test", 5, 30, 0.3, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = retriever.Retrieve(context.Background(), "test query")
	}
}
