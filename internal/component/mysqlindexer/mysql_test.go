package mysqlindexer

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/indexer"
	"github.com/glebarez/sqlite"
	eino_schema "github.com/cloudwego/eino/schema"
	"gorm.io/gorm"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&store.DocumentChunk{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestMySQLIndexer_Store 验证 eino indexer.Store 语义：MetaData 落列、返回 chunk ID。
func TestMySQLIndexer_Store(t *testing.T) {
	db := newTestDB(t)
	idx := NewMySQLIndexer(db)

	docs := []*eino_schema.Document{
		{ID: "c1", Content: "chunk one", MetaData: map[string]any{"document_id": "p1", "chunk_type": "child", "title": "a.md"}},
		{ID: "c2", Content: "chunk two", MetaData: map[string]any{"document_id": "p1", "chunk_type": "parent", "title": "a.md"}},
		// 无 ID / 空 MetaData 的边界：ID 由 indexer 兜底生成，不 panic
		{Content: "chunk three"},
	}

	ids, err := idx.Store(context.Background(), docs)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if len(ids) != 3 || ids[0] != "c1" || ids[2] == "" {
		t.Fatalf("unexpected ids: %v", ids)
	}

	var rows []store.DocumentChunk
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[0].DocID != "p1" || rows[0].ChunkType != "child" || rows[0].Title != "a.md" {
		t.Fatalf("meta not persisted: %+v", rows[0])
	}
	if rows[1].ChunkType != "parent" || rows[2].DocID != "" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

// TestMySQLIndexer_DeleteByDocumentID 验证 VectorDeleter 语义：按父文档 ID 删 chunk 行。
func TestMySQLIndexer_DeleteByDocumentID(t *testing.T) {
	db := newTestDB(t)
	idx, ok := NewMySQLIndexer(db).(interface {
		Store(ctx context.Context, docs []*eino_schema.Document, opts ...indexer.Option) ([]string, error)
		DeleteByDocumentID(ctx context.Context, docID string) error
	})
	if !ok {
		t.Fatal("MySQLIndexer must satisfy service.VectorDeleter for fx type-assertion wiring")
	}
	ctx := context.Background()

	if _, err := idx.Store(ctx, []*eino_schema.Document{
		{ID: "c1", Content: "x", MetaData: map[string]any{"document_id": "p1"}},
		{ID: "c2", Content: "y", MetaData: map[string]any{"document_id": "p2"}},
	}); err != nil {
		t.Fatalf("store: %v", err)
	}

	if err := idx.DeleteByDocumentID(ctx, "p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var rows []store.DocumentChunk
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "c2" {
		t.Fatalf("unexpected remaining: %+v", rows)
	}
}

// TestMySQLIndexer_EmptyStore 空 batch 返回 nil 而非报错。
func TestMySQLIndexer_EmptyStore(t *testing.T) {
	idx := NewMySQLIndexer(newTestDB(t))
	ids, err := idx.Store(context.Background(), nil)
	if err != nil || ids != nil {
		t.Fatalf("empty store: ids=%v err=%v", ids, err)
	}
}
