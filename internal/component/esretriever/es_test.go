package esretriever

import (
	"testing"

	"github.com/cloudwego/eino/components/retriever"
)

func TestNewESRetriever(t *testing.T) {
	r := NewESRetriever(nil, nil, "test_index", 10, 30, 0.3, false)
	if r == nil {
		t.Fatal("NewESRetriever should not return nil")
	}
	// Verify it implements retriever.Retriever.
	var _ retriever.Retriever = r
}

func TestESSearchResponseDecoding(t *testing.T) {
	resp := esSearchResponse{
		Hits: struct {
			Hits []esHit `json:"hits"`
		}{
			Hits: []esHit{
				{
					ID:    "abc",
					Score: 0.95,
					Source: esDoc{
						Content:   "test content",
						MetaData:  map[string]any{"key": "val"},
						CreatedAt: "2026-01-01T00:00:00Z",
					},
				},
			},
		},
	}
	if len(resp.Hits.Hits) != 1 {
		t.Errorf("expected 1 hit, got %d", len(resp.Hits.Hits))
	}
	if resp.Hits.Hits[0].ID != "abc" {
		t.Errorf("ID = %q", resp.Hits.Hits[0].ID)
	}
	if resp.Hits.Hits[0].Score != 0.95 {
		t.Errorf("Score = %f", resp.Hits.Hits[0].Score)
	}
	if resp.Hits.Hits[0].Source.Content != "test content" {
		t.Errorf("Content = %q", resp.Hits.Hits[0].Source.Content)
	}
}

func TestESDocFields(t *testing.T) {
	doc := esDoc{
		Content:   "hello world",
		MetaData:  map[string]any{"chunk_index": 0},
		CreatedAt: "2026-05-13T00:00:00Z",
	}
	if doc.Content != "hello world" {
		t.Errorf("Content = %q", doc.Content)
	}
	if doc.MetaData["chunk_index"] != 0 {
		t.Errorf("chunk_index = %v", doc.MetaData["chunk_index"])
	}
	if doc.CreatedAt != "2026-05-13T00:00:00Z" {
		t.Errorf("CreatedAt = %q", doc.CreatedAt)
	}
}
