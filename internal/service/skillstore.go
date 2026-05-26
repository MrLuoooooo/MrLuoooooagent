package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SkillEntry is a user-defined skill (custom prompt or middleware).
type SkillEntry struct {
	Name    string `json:"name"`
	Prompt  string `json:"prompt"`
	Enabled bool   `json:"enabled"`
}

// SkillStore persists user-added skills to a JSON file.
type SkillStore struct {
	mu     sync.RWMutex
	skills []SkillEntry
	path   string
}

// NewSkillStore loads skills from disk, or creates an empty store.
func NewSkillStore() *SkillStore {
	dir := "data"
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "custom_skills.json")
	store := &SkillStore{path: path, skills: make([]SkillEntry, 0)}
	store.load()
	return store
}

// AddOrUpdate adds or updates a skill and persists to disk.
func (s *SkillStore) AddOrUpdate(entry SkillEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.skills {
		if e.Name == entry.Name {
			s.skills[i] = entry
			return s.save()
		}
	}
	s.skills = append(s.skills, entry)
	return s.save()
}

// Remove deletes a skill by name.
func (s *SkillStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.skills {
		if e.Name == name {
			s.skills = append(s.skills[:i], s.skills[i+1:]...)
			return s.save()
		}
	}
	return fmt.Errorf("skill %q not found", name)
}

// Enabled returns all enabled skills' prompts concatenated.
func (s *SkillStore) Enabled() []SkillEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []SkillEntry
	for _, e := range s.skills {
		if e.Enabled {
			result = append(result, e)
		}
	}
	return result
}

// All returns all skills.
func (s *SkillStore) All() []SkillEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SkillEntry, len(s.skills))
	copy(result, s.skills)
	return result
}

func (s *SkillStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var entries []SkillEntry
	if json.Unmarshal(data, &entries) == nil {
		s.skills = entries
	}
}

func (s *SkillStore) save() error {
	data, err := json.MarshalIndent(s.skills, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
