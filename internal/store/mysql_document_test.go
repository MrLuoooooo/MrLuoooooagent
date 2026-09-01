package store

import (
	"context"
	"testing"
)

// TestMySQLDocumentStore_CRUD 验证文档元数据的落库语义（sqlite 内存库）。
func TestMySQLDocumentStore_CRUD(t *testing.T) {
	db := newTestMySQLDB(t)
	s := NewMySQLDocumentStore(db)
	ctx := context.Background()

	doc := DocumentMeta{ID: "parent_1", Filename: "a.pdf", ChunkCount: 3, CreatedAt: "2026-09-01T00:00:00Z", Content: "raw"}
	if err := s.Save(ctx, doc); err != nil {
		t.Fatalf("save: %v", err)
	}

	// upsert：同 ID 再存覆盖而非报错
	doc.ChunkCount = 5
	if err := s.Save(ctx, doc); err != nil {
		t.Fatalf("resave: %v", err)
	}

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ChunkCount != 5 || list[0].Filename != "a.pdf" {
		t.Fatalf("unexpected list: %+v", list)
	}

	if err := s.Delete(ctx, "parent_1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, err = s.List(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete list=%v err=%v", list, err)
	}
}

// TestDeleteDocumentChunks 验证按父文档 ID 批量删 chunk 行。
func TestDeleteDocumentChunks(t *testing.T) {
	db := newTestMySQLDB(t)

	chunks := []DocumentChunk{
		{ID: "c1", DocID: "p1", ChunkType: "child", Content: "hello"},
		{ID: "c2", DocID: "p1", ChunkType: "parent", Content: "world"},
		{ID: "c3", DocID: "p2", ChunkType: "child", Content: "other"},
	}
	if err := db.Create(&chunks).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := DeleteDocumentChunks(db, "p1"); err != nil {
		t.Fatalf("delete chunks: %v", err)
	}
	var remaining []DocumentChunk
	if err := db.Find(&remaining).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(remaining) != 1 || remaining[0].ID != "c3" {
		t.Fatalf("unexpected remaining: %+v", remaining)
	}
}
