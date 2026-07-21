package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempLabels(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "labels.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRealLabels_Valid(t *testing.T) {
	content := `[
		{"query":"Q1","expected_answer":"A1","must_contain_keywords":["k1","k2"],"category":"finance_factual","difficulty":"easy"}
	]`
	path := writeTempLabels(t, content)
	labels, err := LoadRealLabels(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels) != 1 || labels[0].Query != "Q1" {
		t.Fatalf("expected 1 label, got %v", labels)
	}
}

func TestLoadRealLabels_NotFound(t *testing.T) {
	_, err := LoadRealLabels("/nonexistent/file.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadRealLabels_InvalidJSON(t *testing.T) {
	path := writeTempLabels(t, `{bad json`)
	_, err := LoadRealLabels(path)
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestEvaluateRealLabels_AllPass(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"a", "b"}, Category: "test"},
	}
	retriever := func(q string) []string {
		return []string{"this contains a and b and c"}
	}
	m := EvaluateRealLabels(labels, retriever)
	if m.PassedQueries != 1 {
		t.Fatalf("expected 1 passed, got %d", m.PassedQueries)
	}
	if m.AvgHitRate != 1.0 {
		t.Fatalf("expected hit rate 1.0, got %f", m.AvgHitRate)
	}
}

func TestEvaluateRealLabels_PartialHit(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"alpha", "beta", "gamma", "delta"}, Category: "test"},
	}
	retriever := func(q string) []string {
		return []string{"this text mentions alpha and beta only"} // 2 of 4 → 50%
	}
	m := EvaluateRealLabels(labels, retriever)
	if m.PassedQueries != 1 {
		t.Fatalf("expected pass (50%% of keywords hit), got %d", m.PassedQueries)
	}
	if m.AvgHitRate != 0.5 {
		t.Fatalf("expected 0.5, got %f", m.AvgHitRate)
	}
}

func TestEvaluateRealLabels_Fail(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"alpha", "beta", "gamma"}, Category: "test"},
	}
	retriever := func(q string) []string {
		return []string{"unrelated prose without matching terms"} // 0/3 → 0%
	}
	m := EvaluateRealLabels(labels, retriever)
	if m.PassedQueries != 0 {
		t.Fatalf("expected 0 passed, got %d", m.PassedQueries)
	}
}

func TestEvaluateRealLabels_Empty(t *testing.T) {
	m := EvaluateRealLabels(nil, func(q string) []string { return nil })
	if m.TotalQueries != 0 || m.PassedQueries != 0 {
		t.Fatalf("expected empty, got %v", m)
	}
}

func TestEvaluateRealLabels_MultipleSources(t *testing.T) {
	// 关键词分散在多个 chunk 中也应命中
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"a", "b"}, Category: "test"},
	}
	retriever := func(q string) []string {
		return []string{"chunk 1 with a", "chunk 2 with b"}
	}
	m := EvaluateRealLabels(labels, retriever)
	if m.PassedQueries != 1 {
		t.Fatalf("expected 1 passed (cross-chunk keyword match), got %d", m.PassedQueries)
	}
}

func TestEvaluateRealMetrics_ByCategory(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"a"}, Category: "finance_factual"},
		{Query: "Q2", MustContainKeywords: []string{"b"}, Category: "macro"},
		{Query: "Q3", MustContainKeywords: []string{"c"}, Category: "finance_factual"},
	}
	retriever := func(q string) []string { return []string{} } // 全失败
	m := EvaluateRealLabels(labels, retriever)
	if m.ByCategory["finance_factual"] != 2 || m.ByCategory["macro"] != 1 {
		t.Fatalf("category counts wrong: %v", m.ByCategory)
	}
}
