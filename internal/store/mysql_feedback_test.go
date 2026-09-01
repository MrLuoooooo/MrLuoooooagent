package store

import (
	"context"
	"testing"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"go.uber.org/zap"
)

func TestMySQLFeedbackStore_CRUD(t *testing.T) {
	db := newTestMySQLDB(t)
	s := NewMySQLFeedbackStore(db, zap.NewNop())
	ctx := context.Background()

	now := time.Now().UTC()
	items := []*model.FeedbackItem{
		{ID: "fb_1", ConversationID: "conv_a", MessageIndex: 0, Type: model.FeedbackThumbsUp, Rating: 5, Comment: "good", CreatedAt: now.Add(-time.Minute)},
		{ID: "fb_2", ConversationID: "conv_a", MessageIndex: 1, Type: model.FeedbackCorrection, CorrectAnswer: "42", CreatedAt: now},
		{ID: "fb_3", ConversationID: "conv_b", MessageIndex: 0, Type: model.FeedbackThumbsDown, CreatedAt: now.Add(-time.Hour)},
	}
	for _, it := range items {
		if err := s.Save(ctx, it); err != nil {
			t.Fatalf("save %s: %v", it.ID, err)
		}
	}

	// List 按会话过滤，最新在前
	got, err := s.List(ctx, "conv_a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].ID != "fb_2" || got[1].ID != "fb_1" {
		t.Fatalf("unexpected list order: %+v", ids(got))
	}
	if got[1].Type != model.FeedbackThumbsUp || got[1].Rating != 5 {
		t.Fatalf("fields mismatch: %+v", got[1])
	}

	// ListRecent 全量最近 N 条
	recent, err := s.ListRecent(ctx, 2)
	if err != nil {
		t.Fatalf("list recent: %v", err)
	}
	if len(recent) != 2 || recent[0].ID != "fb_2" || recent[1].ID != "fb_1" {
		t.Fatalf("unexpected recent: %+v", ids(recent))
	}
}

func TestMemFeedbackStore(t *testing.T) {
	s := NewMemFeedbackStore()
	ctx := context.Background()
	if err := s.Save(ctx, &model.FeedbackItem{ID: "a", ConversationID: "c1"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.Save(ctx, &model.FeedbackItem{ID: "b", ConversationID: "c1"}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.List(ctx, "c1")
	if err != nil || len(got) != 2 {
		t.Fatalf("list want 2, got %d err %v", len(got), err)
	}
	recent, err := s.ListRecent(ctx, 1)
	if err != nil || len(recent) != 1 || recent[0].ID != "b" {
		t.Fatalf("list recent want [b], got %+v err %v", ids(recent), err)
	}
}

func ids(items []*model.FeedbackItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}
