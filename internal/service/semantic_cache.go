package service

import (
	"container/list"
	"context"
	"math"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/embedding"
)

// 语义缓存：query embedding → 余弦相似度命中相似问法 → 直接返回缓存答案。
// 与精确缓存（map[key]）的区别：相似问法（"分析茅台" vs "茅台怎么样"）也能命中，
// 本质是"向量空间的最近邻"，面试可对比讲解。
//
// 一致性控制：TTL 过期 + LRU 容量淘汰；实时类问题（股票代码/时效词）跳过缓存。
// 纯内存实现，无 DB；依赖 embedding.Embedder 接口（不依赖具体 provider）。
type SemanticCache struct {
	mu        sync.Mutex
	enabled   bool
	threshold float64
	capacity  int
	ttl       time.Duration
	emb       embedding.Embedder

	entries map[string]*cacheEntry // key = query 原文
	order   *list.List             // LRU 顺序，front = 最近使用
	hits    atomic.Int64
	misses  atomic.Int64
}

type cacheEntry struct {
	query    string
	answer   string
	embed    []float64
	expireAt time.Time
	element  *list.Element
}

// NewSemanticCache 构造语义缓存。emb 为 nil 或 enabled=false 时所有操作退化为 miss。
func NewSemanticCache(emb embedding.Embedder, enabled bool, threshold float64, capacity int, ttl time.Duration) *SemanticCache {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.92
	}
	if capacity <= 0 {
		capacity = 1024
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &SemanticCache{
		enabled:   enabled,
		threshold: threshold,
		capacity:  capacity,
		ttl:       ttl,
		emb:       emb,
		entries:   make(map[string]*cacheEntry),
		order:     list.New(),
	}
}

// Get 命中返回缓存答案；未命中或不可用返回 ("", false)。
func (c *SemanticCache) Get(ctx context.Context, query string) (string, bool) {
	if !c.enabled || c.emb == nil || isRealtimeQuery(query) {
		c.misses.Add(1)
		return "", false
	}
	qvec, err := c.embedQuery(ctx, query)
	if err != nil {
		c.misses.Add(1)
		return "", false
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpiredLocked()

	var best *cacheEntry
	var bestScore float64
	for _, e := range c.entries {
		score := cosine(qvec, e.embed)
		if score > bestScore {
			bestScore = score
			best = e
		}
	}
	if best == nil || bestScore < c.threshold {
		c.misses.Add(1)
		return "", false
	}

	// 命中：LRU 移到 front
	c.order.MoveToFront(best.element)
	c.hits.Add(1)
	return best.answer, true
}

// Put 写入缓存。同 query 覆盖旧条目；超容量淘汰最久未用。
func (c *SemanticCache) Put(ctx context.Context, query, answer string) {
	if !c.enabled || c.emb == nil || query == "" || answer == "" || isRealtimeQuery(query) {
		return
	}
	qvec, err := c.embedQuery(ctx, query)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.evictExpiredLocked()

	if old, ok := c.entries[query]; ok {
		old.answer = answer
		old.embed = qvec
		old.expireAt = time.Now().Add(c.ttl)
		c.order.MoveToFront(old.element)
		return
	}

	// 容量淘汰
	for c.order.Len() >= c.capacity {
		back := c.order.Back()
		if back == nil {
			break
		}
		evict := back.Value.(*cacheEntry)
		c.order.Remove(back)
		delete(c.entries, evict.query)
	}

	e := &cacheEntry{
		query:    query,
		answer:   answer,
		embed:    qvec,
		expireAt: time.Now().Add(c.ttl),
	}
	e.element = c.order.PushFront(e)
	c.entries[query] = e
}

// Stats 返回命中/未命中计数，供日志与监控。
func (c *SemanticCache) Stats() (hits, misses int64) {
	return c.hits.Load(), c.misses.Load()
}

func (c *SemanticCache) embedQuery(ctx context.Context, query string) ([]float64, error) {
	vecs, err := c.emb.EmbedStrings(ctx, []string{query})
	if err != nil || len(vecs) == 0 {
		return nil, err
	}
	return vecs[0], nil
}

func (c *SemanticCache) evictExpiredLocked() {
	now := time.Now()
	for _, e := range c.entries {
		if now.After(e.expireAt) {
			c.order.Remove(e.element)
			delete(c.entries, e.query)
		}
	}
}

// cosine 计算余弦相似度（向量长度不等视为 0）。
func cosine(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

var (
	stockCodeRe = regexp.MustCompile(`(?i)(sh|sz|bj)\d{6}|\d{6}\.(SH|SZ|BJ)`)
	realtimeWords = []string{"现在", "最新", "当前", "今天", "实时", "涨跌", "现价", "行情"}
)

// isRealtimeQuery 判断问题是否强实时性（股票代码或时效词），命中则跳过缓存防过期数据。
func isRealtimeQuery(query string) bool {
	if stockCodeRe.MatchString(query) {
		return true
	}
	for _, w := range realtimeWords {
		if strings.Contains(query, w) {
			return true
		}
	}
	return false
}
