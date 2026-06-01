package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// stubModelSwitcher implements ModelSwitcher for testing.
type stubModelSwitcher struct {
	current string
}

func (s *stubModelSwitcher) Switch(name string) error {
	if name == "error-model" {
		return errStub("switch failed")
	}
	s.current = name
	return nil
}
func (s *stubModelSwitcher) CurrentName() string { return s.current }

type errStub string

func (e errStub) Error() string { return string(e) }

func newModelHandler() *ModelHandler {
	cfg := &config.Config{
		ModelProvider: config.ModelProviderConfig{
			ModelList: []config.ModelEntry{
				{Name: "gpt-4", Provider: "openai", ChatModel: "gpt-4"},
				{Name: "deepseek", Provider: "deepseek", ChatModel: "deepseek-chat"},
			},
		},
	}
	store := service.NewModelStoreWithData([]config.ModelEntry{
		{Name: "custom-1", Provider: "custom", ChatModel: "custom-model"},
	})
	return NewModelHandler(cfg, &stubModelSwitcher{current: "gpt-4"}, store, zap.NewNop())
}

func TestListAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newModelHandler()
	r := gin.New()
	r.GET("/models", h.ListAvailable)

	req := httptest.NewRequest("GET", "/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	if env.Code != 0 {
		t.Fatalf("API code = %d", env.Code)
	}
	data, _ := json.Marshal(env.Data)
	var items []modelItem
	json.Unmarshal(data, &items)
	if len(items) < 2 {
		t.Fatalf("expected >= 2 models, got %d", len(items))
	}
	found := false
	for _, m := range items {
		if m.Name == "gpt-4" && m.Active {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("gpt-4 should be active")
	}
}

func TestListAvailable_DefaultActive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		ModelProvider: config.ModelProviderConfig{
			ModelList: []config.ModelEntry{
				{Name: "gpt-3.5", Provider: "openai", ChatModel: "gpt-3.5"},
			},
		},
	}
	store := service.NewModelStoreWithData(nil)
	h := NewModelHandler(cfg, &stubModelSwitcher{current: "gpt-3.5"}, store, zap.NewNop())
	r := gin.New()
	r.GET("/models", h.ListAvailable)

	req := httptest.NewRequest("GET", "/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := json.Marshal(env.Data)
	var items []modelItem
	json.Unmarshal(data, &items)
	if len(items) == 0 {
		t.Fatal("expected at least 1 model")
	}
	foundConfig := false
	for _, m := range items {
		if m.Name == "gpt-3.5" {
			foundConfig = true
			if !m.Active {
				t.Error("expected gpt-3.5 to be active")
			}
			if m.IsCustom {
				t.Error("expected is_custom = false for config models")
			}
		}
	}
	if !foundConfig {
		t.Error("gpt-3.5 should be in the list")
	}
}

func TestSwitchModel_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newModelHandler()
	r := gin.New()
	r.POST("/models/switch", h.SwitchModel)

	body := bytes.NewReader([]byte(`{"model":"deepseek"}`))
	req := httptest.NewRequest("POST", "/models/switch", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestSwitchModel_MissingField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newModelHandler()
	r := gin.New()
	r.POST("/models/switch", h.SwitchModel)

	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest("POST", "/models/switch", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSwitchModel_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newModelHandler()
	r := gin.New()
	r.POST("/models/switch", h.SwitchModel)

	body := bytes.NewReader([]byte(`{"model":"error-model"}`))
	req := httptest.NewRequest("POST", "/models/switch", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestAddCustomModel_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newModelHandler()
	r := gin.New()
	r.POST("/models", h.AddCustomModel)

	modelName := fmt.Sprintf("new-model-%d", time.Now().UnixNano())
	body := bytes.NewReader([]byte(`{"name":"` + modelName + `","chat_model":"new-chat","provider":"test"}`))
	req := httptest.NewRequest("POST", "/models", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestAddCustomModel_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newModelHandler()
	r := gin.New()
	r.POST("/models", h.AddCustomModel)

	body := bytes.NewReader([]byte(`{"name":"","chat_model":""}`))
	req := httptest.NewRequest("POST", "/models", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestRemoveCustomModel_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewModelStoreWithData([]config.ModelEntry{
		{Name: "temp-model", ChatModel: "temp", Provider: "test"},
	})
	h := NewModelHandler(&config.Config{}, &stubModelSwitcher{current: ""}, store, zap.NewNop())
	r := gin.New()
	r.DELETE("/models/:name", h.RemoveCustomModel)

	req := httptest.NewRequest("DELETE", "/models/temp-model", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestRemoveCustomModel_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewModelStoreWithData(nil)
	h := NewModelHandler(&config.Config{}, &stubModelSwitcher{current: ""}, store, zap.NewNop())
	r := gin.New()
	r.DELETE("/models/:name", h.RemoveCustomModel)

	req := httptest.NewRequest("DELETE", "/models/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
