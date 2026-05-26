package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
)

// ModelStore persists custom user-added models to a JSON file and keeps
// them in memory for fast access. Custom models are merged with config model_list.
type ModelStore struct {
	mu     sync.RWMutex
	custom []config.ModelEntry
	path   string
}

// NewModelStore 从磁盘加载自定义模型，没有就建空的。
func NewModelStore() *ModelStore {
	dir := "data"
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "custom_models.json")
	store := &ModelStore{path: path, custom: make([]config.ModelEntry, 0)}
	store.load()
	return store
}

// Add inserts a new custom model and persists to disk.
func (s *ModelStore) Add(entry config.ModelEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.custom {
		if e.Name == entry.Name {
			return fmt.Errorf("model %q already exists", entry.Name)
		}
	}
	s.custom = append(s.custom, entry)
	return s.save()
}

// Remove deletes a custom model by name.
func (s *ModelStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.custom {
		if e.Name == name {
			s.custom = append(s.custom[:i], s.custom[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("model %q not found", name)
}

// All 列全部自定义模型。
func (s *ModelStore) All() []config.ModelEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]config.ModelEntry, len(s.custom))
	copy(result, s.custom)
	return result
}

func (s *ModelStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var entries []config.ModelEntry
	if json.Unmarshal(data, &entries) == nil {
		s.custom = entries
	}
}

func (s *ModelStore) save() error {
	data, err := json.MarshalIndent(s.custom, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
