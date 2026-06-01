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
	"github.com/MrLuoooooo/MrLuoooooagent/internal/pipeline"
	"go.uber.org/zap"
)

type stubAgentGraph struct{}

func (s *stubAgentGraph) Invoke(_ context.Context, msg *schema.Message, _ ...compose.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "result: " + msg.Content}, nil
}
func (s *stubAgentGraph) Stream(_ context.Context, _ *schema.Message, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}
func (s *stubAgentGraph) Collect(_ context.Context, _ *schema.StreamReader[*schema.Message], _ ...compose.Option) (*schema.Message, error) {
	return nil, nil
}
func (s *stubAgentGraph) Transform(_ context.Context, _ *schema.StreamReader[*schema.Message], _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func newBatchHandler() *BatchHandler {
	p := pipeline.NewBatchPipeline(&stubAgentGraph{})
	return NewBatchHandler(p, zap.NewNop())
}

func TestBatchHandler_EmptyTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newBatchHandler()
	r := gin.New()
	r.POST("/batch", h.HandleBatch)

	body := bytes.NewReader([]byte(`{"tasks":[]}`))
	req := httptest.NewRequest("POST", "/batch", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBatchHandler_TooManyTasks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newBatchHandler()
	r := gin.New()
	r.POST("/batch", h.HandleBatch)

	tasks := make([]map[string]string, 11)
	for i := 0; i < 11; i++ {
		tasks[i] = map[string]string{"id": "t", "prompt": "p"}
	}
	reqBody, _ := json.Marshal(map[string]any{"tasks": tasks})
	body := bytes.NewReader(reqBody)
	req := httptest.NewRequest("POST", "/batch", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBatchHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newBatchHandler()
	r := gin.New()
	r.POST("/batch", h.HandleBatch)

	body := bytes.NewReader([]byte(`not json`))
	req := httptest.NewRequest("POST", "/batch", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
