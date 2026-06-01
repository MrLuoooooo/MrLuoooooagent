package service

import (
	"os"
	"path/filepath"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
)

// NewModelStoreWithData creates a ModelStore with pre-loaded custom models and a temp data directory.
// This is a test helper but placed in a non-test file so it can be used by handler tests.
func NewModelStoreWithData(models []config.ModelEntry) *ModelStore {
	dir := filepath.Join(os.TempDir(), "gptest_models")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "custom_models.json")
	return &ModelStore{
		custom: models,
		path:   path,
	}
}

// NewSkillStoreWithData creates a SkillStore with pre-loaded skills and a temp data directory.
// This is a test helper but placed in a non-test file so it can be used by handler tests.
func NewSkillStoreWithData(skills []SkillEntry) *SkillStore {
	dir := filepath.Join(os.TempDir(), "gptest_skills")
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "custom_skills.json")
	return &SkillStore{
		skills: skills,
		path:   path,
	}
}
