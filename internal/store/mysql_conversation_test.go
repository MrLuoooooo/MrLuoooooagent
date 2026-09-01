package store

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newTestMySQLDB 用内存 sqlite 模拟 MySQL，验证 GORM 层的 CRUD 语义。
func newTestMySQLDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1) // 内存库单连接，保证多语句看到同一库
	if err := AutoMigrateMySQL(db); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestMySQLConversationStore_CRUD(t *testing.T) {
	db := newTestMySQLDB(t)
	s := NewMySQLConversationStore(db, zap.NewNop())
	ctx := context.Background()

	if err := s.Create(ctx, "conv_1", "hello"); err != nil {
		t.Fatalf("create: %v", err)
	}

	metas, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(metas) != 1 || metas[0].ID != "conv_1" || metas[0].MessageCount != 0 {
		t.Fatalf("unexpected metas: %+v", metas)
	}

	msgs := []*schema.Message{
		{Role: schema.User, Content: "hi"},
		{Role: schema.Assistant, Content: "yo"},
	}
	if err := s.Save(ctx, "conv_1", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := s.Load(ctx, "conv_1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 || loaded[0].Content != "hi" || loaded[1].Content != "yo" {
		t.Fatalf("unexpected loaded messages: %+v", loaded)
	}

	metas, _ = s.List(ctx)
	if metas[0].MessageCount != 2 {
		t.Fatalf("message_count want 2, got %d", metas[0].MessageCount)
	}

	if err := s.UpdateTitle(ctx, "conv_1", "renamed"); err != nil {
		t.Fatalf("update title: %v", err)
	}
	metas, _ = s.List(ctx)
	if metas[0].Title != "renamed" {
		t.Fatalf("title not updated: %+v", metas[0])
	}
}

func TestMySQLConversationStore_EmptySave(t *testing.T) {
	db := newTestMySQLDB(t)
	s := NewMySQLConversationStore(db, zap.NewNop())
	ctx := context.Background()
	if err := s.Save(ctx, "conv_x", nil); err != nil {
		t.Fatalf("empty save should be no-op, got: %v", err)
	}
}

func TestMySQLConversationStore_ToolCallsRoundTrip(t *testing.T) {
	db := newTestMySQLDB(t)
	s := NewMySQLConversationStore(db, zap.NewNop())
	ctx := context.Background()
	if err := s.Create(ctx, "conv_tc", "t"); err != nil {
		t.Fatalf("create: %v", err)
	}
	msgs := []*schema.Message{{
		Role:    schema.Assistant,
		Content: "checking",
		ToolCalls: []schema.ToolCall{{
			ID:   "call_1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "web_search",
				Arguments: `{"q":"go"}`,
			},
		}},
	}}
	if err := s.Save(ctx, "conv_tc", msgs); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := s.Load(ctx, "conv_tc")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 1 || len(loaded[0].ToolCalls) != 1 {
		t.Fatalf("toolcalls not roundtripped: %+v", loaded)
	}
	if loaded[0].ToolCalls[0].Function.Name != "web_search" ||
		loaded[0].ToolCalls[0].Function.Arguments != `{"q":"go"}` {
		t.Fatalf("toolcall fields mismatch: %+v", loaded[0].ToolCalls[0])
	}
}

func TestMySQLConversationStore_SaveUnknownConvRollsBack(t *testing.T) {
	db := newTestMySQLDB(t)
	s := NewMySQLConversationStore(db, zap.NewNop())
	ctx := context.Background()

	// 不 Create 直接 Save：事务必须回滚，消息不得落库（孤儿数据防护）。
	if err := s.Save(ctx, "conv_missing", []*schema.Message{{Role: schema.User, Content: "x"}}); err == nil {
		t.Fatal("expected error for unknown conversation")
	}
	loaded, err := s.Load(ctx, "conv_missing")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("messages should be rolled back, got %d", len(loaded))
	}
}

func TestMySQLConversationStore_DeleteAndDeleteAll(t *testing.T) {
	db := newTestMySQLDB(t)
	s := NewMySQLConversationStore(db, zap.NewNop())
	ctx := context.Background()

	_ = s.Create(ctx, "conv_a", "a")
	_ = s.Create(ctx, "conv_b", "b")
	_ = s.Save(ctx, "conv_a", []*schema.Message{{Role: schema.User, Content: "m1"}})

	if err := s.Delete(ctx, "conv_a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	metas, _ := s.List(ctx)
	if len(metas) != 1 || metas[0].ID != "conv_b" {
		t.Fatalf("after delete want only conv_b, got %+v", metas)
	}
	if loaded, _ := s.Load(ctx, "conv_a"); len(loaded) != 0 {
		t.Fatalf("messages should be deleted, got %d", len(loaded))
	}

	if err := s.DeleteAll(ctx); err != nil {
		t.Fatalf("delete all: %v", err)
	}
	metas, _ = s.List(ctx)
	if len(metas) != 0 {
		t.Fatalf("want empty list, got %+v", metas)
	}
}
