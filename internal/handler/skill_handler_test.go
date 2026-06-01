package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

func newSkillHandler() *SkillHandler {
	store := service.NewSkillStore()
	return NewSkillHandler(store, zap.NewNop())
}

func TestSkillList_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newSkillHandler()
	r := gin.New()
	r.GET("/skills", h.List)

	req := httptest.NewRequest("GET", "/skills", nil)
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

func TestSkillList_WithItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewSkillStore()
	store.AddOrUpdate(service.SkillEntry{Name: "test-skill", Prompt: "do something", Enabled: true})
	h := NewSkillHandler(store, zap.NewNop())
	r := gin.New()
	r.GET("/skills", h.List)

	req := httptest.NewRequest("GET", "/skills", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	data, _ := json.Marshal(env.Data)
	var skills []service.SkillEntry
	json.Unmarshal(data, &skills)
	if len(skills) != 1 || skills[0].Name != "test-skill" {
		t.Errorf("unexpected skills: %+v", skills)
	}
}

func TestSkillUpsert_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newSkillHandler()
	r := gin.New()
	r.POST("/skills", h.Upsert)

	body := bytes.NewReader([]byte(`{"name":"my-skill","prompt":"do work","enabled":true}`))
	req := httptest.NewRequest("POST", "/skills", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestSkillUpsert_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newSkillHandler()
	r := gin.New()
	r.POST("/skills", h.Upsert)

	tests := []string{
		`{"name":"","prompt":"work"}`,
		`{"name":"my-skill","prompt":""}`,
		`{}`,
	}
	for _, bodyStr := range tests {
		body := bytes.NewReader([]byte(bodyStr))
		req := httptest.NewRequest("POST", "/skills", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("body=%q: status=%d, want 400", bodyStr, w.Code)
		}
	}
}

func TestSkillUpsert_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newSkillHandler()
	r := gin.New()
	r.POST("/skills", h.Upsert)

	body := bytes.NewReader([]byte(`not json`))
	req := httptest.NewRequest("POST", "/skills", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestSkillRemove_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := service.NewSkillStore()
	store.AddOrUpdate(service.SkillEntry{Name: "my-skill", Prompt: "do it"})
	h := NewSkillHandler(store, zap.NewNop())
	r := gin.New()
	r.DELETE("/skills/:name", h.Remove)

	req := httptest.NewRequest("DELETE", "/skills/my-skill", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestSkillRemove_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newSkillHandler()
	r := gin.New()
	r.DELETE("/skills/:name", h.Remove)

	req := httptest.NewRequest("DELETE", "/skills/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
