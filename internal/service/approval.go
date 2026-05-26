package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

const maxApprovalItems = 1000

// ApprovalStore is a thread-safe in-memory store for approval items.
// Shared between CronScheduler (cron-triggered) and ChatHandler (stream-triggered).
type ApprovalStore struct {
	mu       sync.RWMutex
	items    []*model.ApprovalItem
	maxItems int
}

// NewApprovalStore creates an empty ApprovalStore.
func NewApprovalStore() *ApprovalStore {
	return &ApprovalStore{items: make([]*model.ApprovalItem, 0), maxItems: maxApprovalItems}
}

// Add inserts a new approval item.
func (s *ApprovalStore) Add(item *model.ApprovalItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= s.maxItems {
		kept := s.items[len(s.items)-s.maxItems/2:]
		s.items = make([]*model.ApprovalItem, len(kept))
		copy(s.items, kept)
	}
	s.items = append(s.items, item)
}

// Pending returns all pending approval items.
func (s *ApprovalStore) Pending() []*model.ApprovalItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pending := make([]*model.ApprovalItem, 0)
	for _, a := range s.items {
		if a.Status == model.ApprovalPending {
			pending = append(pending, a)
		}
	}
	return pending
}

// All returns all approval items (including processed ones).
func (s *ApprovalStore) All() []*model.ApprovalItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*model.ApprovalItem, len(s.items))
	copy(result, s.items)
	return result
}

// Get returns a single approval item by ID.
func (s *ApprovalStore) Get(id string) (*model.ApprovalItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.items {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, fmt.Errorf("审批项 %s 不存在", id)
}

// Decide accepts or rejects a pending approval.
func (s *ApprovalStore) Decide(id string, accept bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.items {
		if a.ID == id {
			if a.Status != model.ApprovalPending {
				return fmt.Errorf("审批项 %s 状态不是 pending", id)
			}
			now := time.Now()
			if accept {
				a.Status = model.ApprovalAccepted
			} else {
				a.Status = model.ApprovalRejected
			}
			a.ApprovedAt = &now
			return nil
		}
	}
	return fmt.Errorf("审批项 %s 不存在", id)
}
