package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"go.uber.org/zap"
)

// stubConversationStore implements service.ConversationStore for testing.
type stubConversationStore struct {
	creates  map[string]string // id -> title
	messages map[string][]*schema.Message
}

func newStubConvStore() *stubConversationStore {
	return &stubConversationStore{
		creates:  make(map[string]string),
		messages: make(map[string][]*schema.Message),
	}
}

func (s *stubConversationStore) Create(_ context.Context, id, title string) error {
	s.creates[id] = title
	return nil
}

func (s *stubConversationStore) List(_ context.Context) ([]store.ConversationMeta, error) {
	return nil, nil
}

func (s *stubConversationStore) Load(_ context.Context, conversationID string) ([]*schema.Message, error) {
	return s.messages[conversationID], nil
}

func (s *stubConversationStore) Save(_ context.Context, conversationID string, msgs []*schema.Message) error {
	s.messages[conversationID] = msgs
	return nil
}

func (s *stubConversationStore) Delete(_ context.Context, conversationID string) error {
	delete(s.creates, conversationID)
	delete(s.messages, conversationID)
	return nil
}

func (s *stubConversationStore) UpdateTitle(_ context.Context, id, title string) error {
	if _, ok := s.creates[id]; ok {
		s.creates[id] = title
	}
	return nil
}

func (s *stubConversationStore) DeleteAll(_ context.Context) error {
	s.creates = make(map[string]string)
	s.messages = make(map[string][]*schema.Message)
	return nil
}

func setupConvTest(t *testing.T) (*gin.Engine, *ConversationHandler, *stubConversationStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	stub := newStubConvStore()
	svc := service.NewConversationService(stub, logger)
	h := NewConversationHandler(svc, logger)
	r := gin.New()
	return r, h, stub
}

func TestCreateConversation_DefaultTitle(t *testing.T) {
	r, h, _ := setupConvTest(t)
	r.POST("/conversations", h.CreateConversation)

	req := httptest.NewRequest("POST", "/conversations", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var env model.APIEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("API code = %d, msg = %s", env.Code, env.Message)
	}
}

func TestCreateConversation_WithTitle(t *testing.T) {
	r, h, _ := setupConvTest(t)
	r.POST("/conversations", h.CreateConversation)

	body := model.CreateConversationRequest{Title: "my test"}
	data, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/conversations", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	respData, _ := json.Marshal(env.Data)
	var resp model.CreateConversationResponse
	json.Unmarshal(respData, &resp)
	if resp.Title != "my test" {
		t.Errorf("title = %q, want 'my test'", resp.Title)
	}
	if resp.ConversationID == "" {
		t.Errorf("expected non-empty conversation_id")
	}
}

func TestListConversations_Empty(t *testing.T) {
	r, h, _ := setupConvTest(t)
	r.GET("/conversations", h.ListConversations)

	req := httptest.NewRequest("GET", "/conversations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 0 {
		t.Errorf("code = %d", env.Code)
	}
}

func TestGetMessages_EmptyConv(t *testing.T) {
	r, h, _ := setupConvTest(t)
	r.GET("/conversations/:conversation_id/messages", h.GetMessages)

	req := httptest.NewRequest("GET", "/conversations/conv-1/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := json.Marshal(env.Data)
	var resp model.GetMessagesResponse
	json.Unmarshal(data, &resp)
	if resp.Total != 0 {
		t.Errorf("expected 0 messages, got %d", resp.Total)
	}
}

func TestGetMessages_WithHistory(t *testing.T) {
	r, h, stub := setupConvTest(t)
	r.GET("/conversations/:conversation_id/messages", h.GetMessages)

	stub.messages["conv-1"] = []*schema.Message{
		{Role: schema.User, Content: "hi"},
		{Role: schema.Assistant, Content: "hello"},
	}

	req := httptest.NewRequest("GET", "/conversations/conv-1/messages", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := json.Marshal(env.Data)
	var resp model.GetMessagesResponse
	json.Unmarshal(data, &resp)
	if resp.Total != 2 || len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", resp.Total)
	}
	if resp.Messages[0].Role != "user" || resp.Messages[1].Role != "assistant" {
		t.Errorf("roles: %s, %s", resp.Messages[0].Role, resp.Messages[1].Role)
	}
}

func TestDeleteConversation(t *testing.T) {
	r, h, _ := setupConvTest(t)
	r.DELETE("/conversations/:conversation_id", h.DeleteConversation)

	req := httptest.NewRequest("DELETE", "/conversations/conv-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 0 {
		t.Errorf("code = %d", env.Code)
	}
}

func TestDeleteAllConversations(t *testing.T) {
	r, h, _ := setupConvTest(t)
	r.DELETE("/conversations", h.DeleteAllConversations)

	req := httptest.NewRequest("DELETE", "/conversations", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 0 {
		t.Errorf("code = %d", env.Code)
	}
}
