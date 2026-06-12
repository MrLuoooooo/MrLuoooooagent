package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

const maxApprovalItems = 1000
const approvalTTL = 30 * 24 * time.Hour // 已处理审批项的保留时间

// ApprovalStore 审批项存储——内存 + JSON 文件持久化，重启不丢，自动清理过期项。
type ApprovalStore struct {
	mu       sync.RWMutex
	items    []*model.ApprovalItem
	path     string
	maxItems int
}

// NewApprovalStore 从 dataDir 加载审批项。
func NewApprovalStore(dataDir string) *ApprovalStore {
	path := filepath.Join(dataDir, "approvals.json")
	s := &ApprovalStore{
		path:     path,
		maxItems: maxApprovalItems,
		items:    make([]*model.ApprovalItem, 0),
	}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &s.items)
	}
	return s
}

func (s *ApprovalStore) save() {
	s.purgeExpired()
	dir := filepath.Dir(s.path)
	os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(s.items, "", "  ")
	os.WriteFile(s.path, data, 0644)
}

// purgeExpired 清理超过 TTL 的已处理审批项（pending 永不过期）。
func (s *ApprovalStore) purgeExpired() {
	cutoff := time.Now().Add(-approvalTTL)
	kept := s.items[:0]
	for _, item := range s.items {
		if item.Status == model.ApprovalPending {
			kept = append(kept, item)
			continue
		}
		// 已处理 → 有 ApprovedAt 且超过 TTL 才清理。
		if item.ApprovedAt != nil && item.ApprovedAt.Before(cutoff) {
			continue // 丢弃
		}
		// ApprovedAt 未设置（旧数据）→ 检查 CreatedAt。
		if item.ApprovedAt == nil && !item.CreatedAt.IsZero() && item.CreatedAt.Before(cutoff) {
			continue
		}
		kept = append(kept, item)
	}
	s.items = kept
}

// Add 插入一条审批项。
func (s *ApprovalStore) Add(item *model.ApprovalItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= s.maxItems {
		kept := s.items[len(s.items)-s.maxItems/2:]
		s.items = make([]*model.ApprovalItem, len(kept))
		copy(s.items, kept)
	}
	s.items = append(s.items, item)
	s.save()
}

// Pending 列出待审批的。
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

// All 列出全部审批项。
func (s *ApprovalStore) All() []*model.ApprovalItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*model.ApprovalItem, len(s.items))
	copy(result, s.items)
	return result
}

// Get 按 ID 取一个审批项。
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

// Decide 审批决定（pended → accepted/rejected）。
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
			s.save()
			return nil
		}
	}
	return fmt.Errorf("审批项 %s 不存在", id)
}
