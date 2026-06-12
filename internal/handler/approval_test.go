package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/scheduler"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

type stubAgent struct{}

func (s *stubAgent) Invoke(_ context.Context, _ *schema.Message, _ ...compose.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "stub"}, nil
}
func (s *stubAgent) Stream(_ context.Context, _ *schema.Message, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}
func (s *stubAgent) Collect(_ context.Context, _ *schema.StreamReader[*schema.Message], _ ...compose.Option) (*schema.Message, error) {
	return nil, nil
}
func (s *stubAgent) Transform(_ context.Context, _ *schema.StreamReader[*schema.Message], _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func setupApprovalTest(t *testing.T) (*gin.Engine, *zap.Logger) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	cfg := &config.Config{Cron: config.CronConfig{Enabled: false}}
	_ = scheduler.NewCronScheduler(cfg, &stubAgent{}, logger, nil)
	store := service.NewApprovalStore("data")
	h := NewApprovalHandler(store, logger)

	r := gin.New()
	r.GET("/approvals/pending", h.ListPending)
	r.GET("/approvals", h.ListAll)
	r.POST("/approvals/:approval_id/decide", h.Decide)
	return r, logger
}

func mustJSON(v any) []byte {
	data, _ := json.Marshal(v)
	return data
}

func TestListPending_Empty(t *testing.T) {
	r, _ := setupApprovalTest(t)
	req := httptest.NewRequest("GET", "/approvals/pending", nil)
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
		t.Fatalf("API code = %d, message = %s", env.Code, env.Message)
	}
}

func TestListAll_Empty(t *testing.T) {
	r, _ := setupApprovalTest(t)
	req := httptest.NewRequest("GET", "/approvals", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestDecide_NotFound(t *testing.T) {
	r, _ := setupApprovalTest(t)
	req := httptest.NewRequest("POST", "/approvals/nonexistent/decide",
		bytes.NewReader(mustJSON(map[string]bool{"accept": true})))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestDecide_InvalidJSON(t *testing.T) {
	r, _ := setupApprovalTest(t)
	req := httptest.NewRequest("POST", "/approvals/123/decide",
		bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}
