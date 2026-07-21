package service

import (
	"context"
	"sync"
	"time"
)

// ShortTermCache 短期记忆热缓存接口。
// Redis-List LTRIM 实现是生产推荐；NoOpCache 用于无 Redis 环境（纯 ES 持久化）。
//
// 数据流：handler 写消息 → cache.Push → Redis List
// 读消息：handler 加载历史 → cache.GetWindow → Redis LRANGE
// 兜底：Redis 故障 → 走 ES conversation store（不阻断主流程）
type ShortTermCache interface {
	Push(ctx context.Context, convID string, item ShortTermItem) error
	GetWindow(ctx context.Context, convID string, size int) ([]ShortTermItem, error)
	Trim(ctx context.Context, convID string, maxLen int) error
	Close() error
}

// ShortTermItem 单条会话消息缓存。
type ShortTermItem struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// NoOpCache 内存版实现：进程内 sync.Map 维护最近 N 条，无外部依赖。
// 适合开发/测试环境。生产建议用 RedisListCache 替换。
type NoOpCache struct {
	mu       sync.RWMutex
	windows  map[string][]ShortTermItem
	maxLen   int
}

func NewNoOpCache(maxLen int) *NoOpCache {
	return &NoOpCache{
		windows: make(map[string][]ShortTermItem),
		maxLen:  maxLen,
	}
}

func (c *NoOpCache) Push(ctx context.Context, convID string, item ShortTermItem) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.windows[convID] = append(c.windows[convID], item)
	if len(c.windows[convID]) > c.maxLen {
		c.windows[convID] = c.windows[convID][len(c.windows[convID])-c.maxLen:]
	}
	return nil
}

func (c *NoOpCache) GetWindow(ctx context.Context, convID string, size int) ([]ShortTermItem, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	items := c.windows[convID]
	if len(items) > size {
		items = items[len(items)-size:]
	}
	cp := make([]ShortTermItem, len(items))
	copy(cp, items)
	return cp, nil
}

func (c *NoOpCache) Trim(ctx context.Context, convID string, maxLen int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	items := c.windows[convID]
	if len(items) > maxLen {
		c.windows[convID] = items[len(items)-maxLen:]
	}
	return nil
}

func (c *NoOpCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.windows = make(map[string][]ShortTermItem)
	return nil
}
