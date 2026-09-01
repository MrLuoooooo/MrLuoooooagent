package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"go.uber.org/zap"
)

func TestHealthEndpoint(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Mode: "test"},
		Auth:   config.AuthConfig{APIKey: ""},
	}
	logger := zap.NewNop()

	router := NewRouter(cfg, logger, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if router == nil {
		t.Fatal("NewRouter() returned nil")
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health endpoint returned %d, want 200", w.Code)
	}

	expected := `"status":"ok"`
	body := w.Body.String()
	if !contains(body, expected) {
		t.Errorf("health response missing %q: %s", expected, body)
	}
	if !contains(body, `"version":"4.1.2"`) {
		t.Error("health response missing version")
	}
	// verify timestamp is RFC3339
	if !contains(body, time.Now().UTC().Format("2006-01-02")) {
		t.Error("health response timestamp not in expected format")
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
