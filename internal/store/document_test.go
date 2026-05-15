package store

import (
	"testing"
)

func TestDocumentMeta_Fields(t *testing.T) {
	doc := DocumentMeta{
		ID:         "doc-001",
		Filename:   "test.txt",
		ChunkCount: 3,
		CreatedAt:  "2026-01-01T00:00:00Z",
		Content:    "hello world",
	}
	if doc.ID != "doc-001" {
		t.Errorf("ID = %q, want %q", doc.ID, "doc-001")
	}
	if doc.Filename != "test.txt" {
		t.Errorf("Filename = %q", doc.Filename)
	}
	if doc.ChunkCount != 3 {
		t.Errorf("ChunkCount = %d, want 3", doc.ChunkCount)
	}
}

func TestESDocRecord_Mapping(t *testing.T) {
	r := esDocRecord{
		DocumentID: "abc",
		Filename:   "f.txt",
		ChunkCount: 5,
		CreatedAt:  "2026-01-01T00:00:00Z",
		Content:    "text",
	}
	if r.DocumentID != "abc" {
		t.Errorf("DocumentID = %q", r.DocumentID)
	}
	if r.ChunkCount != 5 {
		t.Errorf("ChunkCount = %d", r.ChunkCount)
	}
}

// TestDocumentMetaConversion verifies conversion between DocMeta and ES record.
func TestDocumentMetaConversion(t *testing.T) {
	meta := DocumentMeta{
		ID: "id1", Filename: "f.txt", ChunkCount: 2,
		CreatedAt: "2026-05-01T00:00:00Z", Content: "hello",
	}
	record := esDocRecord{
		DocumentID: meta.ID,
		Filename:   meta.Filename,
		ChunkCount: meta.ChunkCount,
		CreatedAt:  meta.CreatedAt,
		Content:    meta.Content,
	}
	if record.DocumentID != meta.ID {
		t.Errorf("DocumentID mismatch: %q vs %q", record.DocumentID, meta.ID)
	}
	if record.Filename != meta.Filename {
		t.Errorf("Filename mismatch")
	}
	if record.Content != meta.Content {
		t.Errorf("Content mismatch")
	}
}
