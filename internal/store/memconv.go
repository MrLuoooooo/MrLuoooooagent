package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
)

// MemConvStore 内存会话存储，ES 不可用时的降级方案。
type MemConvStore struct {
	mu      sync.RWMutex
	convs   map[string]*memConv
	nextID  int
}

type memConv struct {
	ID        string
	Title     string
	Messages  []*schema.Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewMemConvStore —
func NewMemConvStore() *MemConvStore {
	return &MemConvStore{
		convs: make(map[string]*memConv),
	}
}

func (s *MemConvStore) Create(ctx context.Context, id, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.convs[id] = &memConv{ID: id, Title: title, CreatedAt: now, UpdatedAt: now}
	return nil
}

func (s *MemConvStore) List(ctx context.Context) ([]ConversationMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ConversationMeta, 0, len(s.convs))
	for _, c := range s.convs {
		result = append(result, ConversationMeta{
			ID: c.ID, Title: c.Title,
			MessageCount: len(c.Messages),
			CreatedAt:    c.CreatedAt,
			UpdatedAt:    c.UpdatedAt,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt.After(result[j].UpdatedAt) })
	return result, nil
}

func (s *MemConvStore) Load(ctx context.Context, id string) ([]*schema.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.convs[id]
	if !ok {
		return nil, fmt.Errorf("conversation %s not found", id)
	}
	out := make([]*schema.Message, len(c.Messages))
	copy(out, c.Messages)
	return out, nil
}

func (s *MemConvStore) Save(ctx context.Context, id string, msgs []*schema.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[id]
	if !ok {
		return fmt.Errorf("conversation %s not found", id)
	}
	c.Messages = append(c.Messages, msgs...)
	c.UpdatedAt = time.Now()
	return nil
}

func (s *MemConvStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.convs, id)
	return nil
}

func (s *MemConvStore) UpdateTitle(ctx context.Context, id, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.convs[id]
	if !ok {
		return fmt.Errorf("conversation %s not found", id)
	}
	c.Title = title
	c.UpdatedAt = time.Now()
	return nil
}

func (s *MemConvStore) DeleteAll(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.convs = make(map[string]*memConv)
	return nil
}
