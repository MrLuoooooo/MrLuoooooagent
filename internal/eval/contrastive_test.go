package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateContrastiveTriplets_FindPositive(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"alpha", "beta"}},
	}
	corpus := func(q string) []string {
		return []string{
			"this mentions alpha and beta together",
			"this has alpha only",
			"unrelated prose with no keywords", // hard negative
		}
	}
	triplets := GenerateContrastiveTriplets(labels, corpus)
	if len(triplets) != 1 {
		t.Fatalf("expected 1 triplet, got %d", len(triplets))
	}
	if !strings.Contains(triplets[0].Positive, "alpha") || !strings.Contains(triplets[0].Positive, "beta") {
		t.Fatalf("positive not found: %s", triplets[0].Positive)
	}
	if strings.Contains(triplets[0].Negative, "alpha") || strings.Contains(triplets[0].Negative, "beta") {
		t.Fatalf("negative should not contain positive keywords: %s", triplets[0].Negative)
	}
}

func TestGenerateContrastiveTriplets_NoPositive(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"alpha", "beta"}},
	}
	corpus := func(q string) []string {
		return []string{"no relevant content", "more irrelevant"}
	}
	triplets := GenerateContrastiveTriplets(labels, corpus)
	if len(triplets) != 0 {
		t.Fatalf("expected 0 triplets (no positive found), got %d", len(triplets))
	}
}

func TestGenerateContrastiveTriplets_HardNegativeSkipsRelevant(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"alpha"}},
	}
	corpus := func(q string) []string {
		return []string{
			"contains alpha",          // 这是 positive
			"contains alpha and more", // 也含 alpha → 跳过
			"unrelated text",          // hard negative
		}
	}
	triplets := GenerateContrastiveTriplets(labels, corpus)
	if len(triplets) != 1 {
		t.Fatalf("expected 1 triplet, got %d", len(triplets))
	}
	if strings.Contains(triplets[0].Negative, "alpha") {
		t.Fatalf("negative should not contain alpha: %s", triplets[0].Negative)
	}
}

func TestGenerateContrastiveTriplets_MaxThreeNegatives(t *testing.T) {
	labels := []RealQueryLabel{
		{Query: "Q1", MustContainKeywords: []string{"x"}},
	}
	corpus := func(q string) []string {
		return []string{
			"contains x",
			"n1", "n2", "n3", "n4", "n5", // 5 个 hard negative
		}
	}
	triplets := GenerateContrastiveTriplets(labels, corpus)
	if len(triplets) != 3 {
		t.Fatalf("expected 3 triplets (max negatives), got %d", len(triplets))
	}
}

func TestExportTripletsJSONL(t *testing.T) {
	triplets := []ContrastiveTriplet{
		{Query: "Q1", Positive: "P1", Negative: "N1", Score: 4},
		{Query: "Q2", Positive: "P2", Negative: "N2", Score: 4},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "triplets.jsonl")
	if err := ExportTripletsJSONL(triplets, path); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	var got ContrastiveTriplet
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatal(err)
	}
	if got.Query != "Q1" || got.Positive != "P1" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
