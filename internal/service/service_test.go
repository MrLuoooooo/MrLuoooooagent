package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"go.uber.org/zap"
)

// --- ModelStore tests ---

func TestModelStore_Add(t *testing.T) {
	store := NewModelStoreWithData(nil)
	err := store.Add(config.ModelEntry{Name: "test-model", ChatModel: "test-chat", Provider: "test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	all := store.All()
	if len(all) != 1 || all[0].Name != "test-model" {
		t.Errorf("got %+v", all)
	}
}

func TestModelStore_AddDuplicate(t *testing.T) {
	store := NewModelStoreWithData([]config.ModelEntry{{Name: "dup", ChatModel: "x", Provider: "p"}})
	err := store.Add(config.ModelEntry{Name: "dup", ChatModel: "y", Provider: "q"})
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
}

func TestModelStore_Remove(t *testing.T) {
	store := NewModelStoreWithData([]config.ModelEntry{{Name: "m1", ChatModel: "c1", Provider: "p1"}})
	err := store.Remove("m1")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(store.All()) != 0 {
		t.Errorf("expected empty, got %d", len(store.All()))
	}
}

func TestModelStore_RemoveNotFound(t *testing.T) {
	store := NewModelStoreWithData(nil)
	err := store.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestModelStore_All(t *testing.T) {
	entries := []config.ModelEntry{
		{Name: "m1", ChatModel: "c1", Provider: "p1"},
		{Name: "m2", ChatModel: "c2", Provider: "p2"},
	}
	s := NewModelStoreWithData(entries)
	all := s.All()
	if len(all) != 2 {
		t.Fatalf("len = %d", len(all))
	}
	// Verify defensive copy
	all[0].Name = "modified"
	if s.All()[0].Name != "m1" {
		t.Error("All() should return a copy")
	}
}

func TestModelStore_AllEmpty(t *testing.T) {
	s := NewModelStoreWithData(nil)
	all := s.All()
	if len(all) != 0 {
		t.Errorf("expected empty, got %d", len(all))
	}
}

// --- SkillStore tests ---

func TestSkillStore_AddOrUpdate(t *testing.T) {
	s := NewSkillStoreWithData(nil)
	err := s.AddOrUpdate(SkillEntry{Name: "s1", Prompt: "do it", Enabled: true})
	if err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}
	all := s.All()
	if len(all) != 1 || all[0].Name != "s1" {
		t.Errorf("got %+v", all)
	}
}

func TestSkillStore_UpdateExisting(t *testing.T) {
	s := NewSkillStoreWithData([]SkillEntry{{Name: "s1", Prompt: "old", Enabled: false}})
	err := s.AddOrUpdate(SkillEntry{Name: "s1", Prompt: "new", Enabled: true})
	if err != nil {
		t.Fatalf("AddOrUpdate: %v", err)
	}
	all := s.All()
	if len(all) != 1 || all[0].Prompt != "new" {
		t.Errorf("got %+v", all)
	}
}

func TestSkillStore_Remove(t *testing.T) {
	s := NewSkillStoreWithData([]SkillEntry{{Name: "s1", Prompt: "test"}})
	err := s.Remove("s1")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(s.All()) != 0 {
		t.Error("expected empty after remove")
	}
}

func TestSkillStore_RemoveNotFound(t *testing.T) {
	s := NewSkillStoreWithData(nil)
	err := s.Remove("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSkillStore_Enabled(t *testing.T) {
	s := NewSkillStoreWithData([]SkillEntry{
		{Name: "s1", Prompt: "enabled", Enabled: true},
		{Name: "s2", Prompt: "disabled", Enabled: false},
		{Name: "s3", Prompt: "also enabled", Enabled: true},
	})
	enabled := s.Enabled()
	if len(enabled) != 2 {
		t.Fatalf("expected 2 enabled, got %d", len(enabled))
	}
	if enabled[0].Name != "s1" || enabled[1].Name != "s3" {
		t.Errorf("got %+v", enabled)
	}
}

func TestSkillStore_EnabledAllDisabled(t *testing.T) {
	s := NewSkillStoreWithData([]SkillEntry{
		{Name: "s1", Prompt: "d1", Enabled: false},
	})
	enabled := s.Enabled()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled, got %d", len(enabled))
	}
}

func TestSkillStore_AllReturnsCopy(t *testing.T) {
	s := NewSkillStoreWithData([]SkillEntry{{Name: "s1", Prompt: "p1", Enabled: true}})
	all := s.All()
	all[0].Name = "modified"
	if s.All()[0].Name != "s1" {
		t.Error("All() should return a copy")
	}
}

func TestSkillStore_AllEmpty(t *testing.T) {
	s := NewSkillStoreWithData(nil)
	all := s.All()
	if len(all) != 0 {
		t.Errorf("expected empty, got %d", len(all))
	}
}

// --- ApprovalStore tests ---

func cleanApprovalFile(t *testing.T) {
	t.Helper()
	os.Remove("data/approvals.json")
}

func TestApprovalStore_AddAndPending(t *testing.T) {
	cleanApprovalFile(t)
	as := NewApprovalStore("data")
	as.Add(&model.ApprovalItem{ID: "a1", Status: model.ApprovalPending})
	as.Add(&model.ApprovalItem{ID: "a2", Status: model.ApprovalAccepted})
	pending := as.Pending()
	if len(pending) != 1 || pending[0].ID != "a1" {
		t.Errorf("pending: %+v", pending)
	}
}

func TestApprovalStore_All(t *testing.T) {
	cleanApprovalFile(t)
	as := NewApprovalStore("data")
	as.Add(&model.ApprovalItem{ID: "a1", Status: model.ApprovalPending})
	as.Add(&model.ApprovalItem{ID: "a2", Status: model.ApprovalRejected})
	all := as.All()
	if len(all) != 2 {
		t.Fatalf("len = %d", len(all))
	}
}

func TestApprovalStore_GetSuccess(t *testing.T) {
	cleanApprovalFile(t)
	as := NewApprovalStore("data")
	as.Add(&model.ApprovalItem{ID: "find-me", Status: model.ApprovalPending})
	item, err := as.Get("find-me")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if item.ID != "find-me" {
		t.Errorf("id = %q", item.ID)
	}
}

func TestApprovalStore_GetNotFound(t *testing.T) {
	as := NewApprovalStore("data")
	_, err := as.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApprovalStore_DecideAccept(t *testing.T) {
	cleanApprovalFile(t)
	as := NewApprovalStore("data")
	as.Add(&model.ApprovalItem{ID: "a1", Status: model.ApprovalPending})
	if err := as.Decide("a1", true); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	item, _ := as.Get("a1")
	if item.Status != model.ApprovalAccepted {
		t.Errorf("status = %s, want accepted", item.Status)
	}
	if item.ApprovedAt == nil {
		t.Error("expected non-nil ApprovedAt")
	}
}

func TestApprovalStore_DecideReject(t *testing.T) {
	cleanApprovalFile(t)
	as := NewApprovalStore("data")
	as.Add(&model.ApprovalItem{ID: "a1", Status: model.ApprovalPending})
	if err := as.Decide("a1", false); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	item, _ := as.Get("a1")
	if item.Status != model.ApprovalRejected {
		t.Errorf("status = %s, want rejected", item.Status)
	}
}

func TestApprovalStore_DecideAlreadyDecided(t *testing.T) {
	cleanApprovalFile(t)
	as := NewApprovalStore("data")
	as.Add(&model.ApprovalItem{ID: "a1", Status: model.ApprovalAccepted})
	err := as.Decide("a1", true)
	if err == nil {
		t.Fatal("expected error for already decided item")
	}
}

func TestApprovalStore_DecideNotFound(t *testing.T) {
	as := NewApprovalStore("data")
	err := as.Decide("nonexistent", true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestApprovalStore_MaxItems(t *testing.T) {
	as := NewApprovalStore("data")
	// Fill past max
	for i := 0; i < 1050; i++ {
		as.Add(&model.ApprovalItem{ID: "a%03d", Status: model.ApprovalPending})
	}
	all := as.All()
	if len(all) > maxApprovalItems {
		t.Errorf("items %d exceeds max %d", len(all), maxApprovalItems)
	}
}

// --- ConversationService tests ---

type stubConvStore struct {
	createErr  error
	listMetas  []store.ConversationMeta
	listErr    error
	loadMsgs   []*schema.Message
	loadErr    error
	saveErr    error
	deleteErr  error
	deleteAllErr error
}

func (s *stubConvStore) Create(_ context.Context, _, _ string) error { return s.createErr }
func (s *stubConvStore) Exists(_ context.Context, id string) (bool, error) {
	for _, m := range s.listMetas {
		if m.ID == id {
			return true, nil
		}
	}
	return false, s.listErr
}

func (s *stubConvStore) List(_ context.Context) ([]store.ConversationMeta, error) {
	return s.listMetas, s.listErr
}
func (s *stubConvStore) Load(_ context.Context, _ string) ([]*schema.Message, error) {
	return s.loadMsgs, s.loadErr
}
func (s *stubConvStore) Save(_ context.Context, _ string, _ []*schema.Message) error { return s.saveErr }
func (s *stubConvStore) Delete(_ context.Context, _ string) error { return s.deleteErr }
func (s *stubConvStore) UpdateTitle(_ context.Context, _, _ string) error { return nil }
func (s *stubConvStore) DeleteAll(_ context.Context) error { return s.deleteAllErr }

func TestConversationService_Create(t *testing.T) {
	svc := NewConversationService(&stubConvStore{}, zap.NewNop())
	id, err := svc.Create(context.Background(), "test title")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty ID")
	}
}

func TestConversationService_CreateError(t *testing.T) {
	svc := NewConversationService(&stubConvStore{createErr: errStub("fail")}, zap.NewNop())
	_, err := svc.Create(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConversationService_List(t *testing.T) {
	now := "2026-01-01T00:00:00Z"
	svc := NewConversationService(&stubConvStore{
		listMetas: []store.ConversationMeta{
			{ID: "c1", Title: "t1", MessageCount: 2, CreatedAt: timeParse(now), UpdatedAt: timeParse(now)},
		},
	}, zap.NewNop())
	metas, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 1 || metas[0].ID != "c1" {
		t.Errorf("got %+v", metas)
	}
}

func TestConversationService_ListError(t *testing.T) {
	svc := NewConversationService(&stubConvStore{listErr: errStub("fail")}, zap.NewNop())
	_, err := svc.List(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConversationService_LoadMessages(t *testing.T) {
	svc := NewConversationService(&stubConvStore{
		loadMsgs: []*schema.Message{{Role: schema.User, Content: "hi"}},
	}, zap.NewNop())
	msgs, err := svc.LoadMessages(context.Background(), "c1")
	if err != nil {
		t.Fatalf("LoadMessages: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "hi" {
		t.Errorf("got %+v", msgs)
	}
}

func TestConversationService_SaveMessages(t *testing.T) {
	svc := NewConversationService(&stubConvStore{}, zap.NewNop())
	err := svc.SaveMessages(context.Background(), "c1", []*schema.Message{{Role: schema.User, Content: "hello"}})
	if err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}
}

func TestConversationService_Delete(t *testing.T) {
	svc := NewConversationService(&stubConvStore{}, zap.NewNop())
	err := svc.Delete(context.Background(), "c1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestConversationService_Rename(t *testing.T) {
	svc := NewConversationService(&stubConvStore{}, zap.NewNop())
	err := svc.Rename(context.Background(), "c1", "new title")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
}

func TestConversationService_DeleteAll(t *testing.T) {
	svc := NewConversationService(&stubConvStore{}, zap.NewNop())
	err := svc.DeleteAll(context.Background())
	if err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }

func timeParse(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
