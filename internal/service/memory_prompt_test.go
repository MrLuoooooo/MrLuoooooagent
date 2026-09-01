package service

import (
	"strings"
	"testing"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
)

func TestMemoryAgeLabel(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		age  time.Duration
		want string
	}{
		{2 * time.Hour, "今天"},
		{25 * time.Hour, "昨天"},
		{3 * 24 * time.Hour, "3 天前"},
	}
	for _, c := range cases {
		if got := memoryAgeLabel(now.Add(-c.age), now); got != c.want {
			t.Errorf("memoryAgeLabel(%v) = %q, want %q", c.age, got, c.want)
		}
	}
	if got := memoryAgeLabel(now.Add(time.Hour), now); got != "今天" {
		t.Errorf("future timestamp should clamp to 今天, got %q", got)
	}
}

func TestMemoryEntryLine_AgingWarning(t *testing.T) {
	now := time.Now()
	fresh := store.MemoryMeta{
		Content: "用户偏好 Go", MemoryLayer: "L1", Confidence: 0.95,
		Status: "active", UpdatedAt: now.Add(-2 * time.Hour),
	}
	line, layer, ok := memoryEntryLine(fresh, now)
	if !ok || layer != "L1" {
		t.Fatalf("fresh L1 should pass, ok=%v layer=%s", ok, layer)
	}
	if strings.Contains(line, "可能已过期") {
		t.Errorf("fresh memory should not carry aging warning: %s", line)
	}

	stale := fresh
	stale.UpdatedAt = now.Add(-72 * time.Hour)
	line, _, _ = memoryEntryLine(stale, now)
	if !strings.Contains(line, "3 天前写入") || !strings.Contains(line, "⚠️") {
		t.Errorf("old memory should carry aging warning, got: %s", line)
	}
}

func TestMemoryEntryLine_LayerRules(t *testing.T) {
	now := time.Now()
	// L1 低置信度被过滤
	if _, _, ok := memoryEntryLine(store.MemoryMeta{MemoryLayer: "L1", Confidence: 0.8, UpdatedAt: now}, now); ok {
		t.Error("L1 with confidence < 0.9 should be filtered")
	}
	// L2 invalid（valid_until 超窗口）被过滤
	if _, _, ok := memoryEntryLine(store.MemoryMeta{
		MemoryLayer: "L2", Confidence: 0.9, UpdatedAt: now,
		ValidUntil: now.Add(-300 * 24 * time.Hour), // 300 天前过期 > 3*90 天
	}, now); ok {
		t.Error("invalid L2 memory should be filtered")
	}
	// L3 非 active 被过滤
	if _, _, ok := memoryEntryLine(store.MemoryMeta{MemoryLayer: "L3", Status: "superseded", UpdatedAt: now}, now); ok {
		t.Error("non-active L3 should be filtered")
	}
}

func TestBuildMemoryPrompt_LayeredAndAged(t *testing.T) {
	now := time.Now()
	mems := []store.MemoryMeta{
		{ID: "1", Content: "偏好蓝橙渐变", MemoryLayer: "L1", Confidence: 0.95, Status: "active", UpdatedAt: now.Add(-time.Hour)},
		{ID: "2", Content: "项目监听 8080", MemoryLayer: "L2", Confidence: 0.8, Status: "active", UpdatedAt: now.Add(-5 * 24 * time.Hour)},
		{ID: "3", Content: "认为某票会涨", MemoryLayer: "L3", Confidence: 0.7, Status: "active", UpdatedAt: now},
	}
	prompt, dropped := buildMemoryPrompt(mems, now, maxMemoryInjectChars)
	if dropped != 0 {
		t.Errorf("within budget, dropped = %d, want 0", dropped)
	}
	if !strings.Contains(prompt, "偏好与画像") || !strings.Contains(prompt, "偏好蓝橙渐变") {
		t.Errorf("L1 section missing: %s", prompt)
	}
	if !strings.Contains(prompt, "5 天前写入") {
		t.Errorf("aging warning missing for 5-day-old memory: %s", prompt)
	}
	if !strings.Contains(prompt, "分析观点") {
		t.Errorf("L3 section missing: %s", prompt)
	}
}

func TestBuildMemoryPrompt_BudgetDropsLowestPriority(t *testing.T) {
	now := time.Now()
	// L1 置信度高 + 两条 L3 大内容：预算收紧时应丢 L3 保留 L1
	big := strings.Repeat("长", 800) // 800 runes/条
	mems := []store.MemoryMeta{
		{ID: "1", Content: "核心偏好", MemoryLayer: "L1", Confidence: 0.95, Status: "active", UpdatedAt: now},
		{ID: "2", Content: big, MemoryLayer: "L3", Confidence: 0.8, Status: "active", UpdatedAt: now},
		{ID: "3", Content: big, MemoryLayer: "L3", Confidence: 0.7, Status: "active", UpdatedAt: now},
	}
	prompt, dropped := buildMemoryPrompt(mems, now, 1200)
	if dropped == 0 {
		t.Errorf("tight budget should drop entries, got 0")
	}
	if !strings.Contains(prompt, "核心偏好") {
		t.Errorf("L1 should survive budget cut, prompt: %s", prompt)
	}
}

func TestBuildMemoryPrompt_Empty(t *testing.T) {
	if prompt, dropped := buildMemoryPrompt(nil, time.Now(), maxMemoryInjectChars); prompt != "" || dropped != 0 {
		t.Errorf("empty input should return empty, got %q/%d", prompt, dropped)
	}
}
