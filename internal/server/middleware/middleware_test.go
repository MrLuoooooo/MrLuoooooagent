package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestCORS_Wildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(nil))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("origin = %q, want *", origin)
	}
}

func TestCORS_SpecificOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"http://example.com"}))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://example.com" {
		t.Errorf("origin = %q", origin)
	}
}

func TestCORS_OptionsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(nil))
	r.OPTIONS("/test", func(c *gin.Context) { c.String(200, "should not reach") })

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}

func TestCORS_HeadersSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(nil))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Access-Control-Allow-Methods header not set")
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("Access-Control-Allow-Headers header not set")
	}
}

func TestLogger_LogsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Logger(logger))
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestLogger_RecordsStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zap.NewNop()
	r := gin.New()
	r.Use(Logger(logger))
	r.GET("/err", func(c *gin.Context) { c.String(500, "fail") })

	req := httptest.NewRequest("GET", "/err", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(100, 200) // 100 rps, burst 200
	if !rl.allow("127.0.0.1") {
		t.Error("expected allow")
	}
}

func TestRateLimiter_Deny(t *testing.T) {
	rl := NewRateLimiter(0, 0) // No tokens
	if rl.allow("127.0.0.1") {
		t.Error("expected deny")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	rl := NewRateLimiter(1000, 1) // 1 token capacity
	if !rl.allow("127.0.0.1") {
		t.Fatal("first request should be allowed")
	}
	if rl.allow("127.0.0.1") {
		t.Fatal("second request should be denied (burst = 1)")
	}
}

func TestRateLimiter_MultipleIPs(t *testing.T) {
	rl := NewRateLimiter(0, 1)
	if !rl.allow("ip1") {
		t.Fatal("ip1 should be allowed")
	}
	if rl.allow("ip1") {
		t.Fatal("ip1 second should be denied")
	}
	// Different IP should still have its own bucket
	if !rl.allow("ip2") {
		t.Fatal("ip2 should be allowed (separate bucket)")
	}
}

func TestRateLimiter_MiddlewareAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(100, 200)
	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestRateLimiter_MiddlewareDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(0, 0) // No tokens for anyone
	r := gin.New()
	r.Use(rl.Middleware())
	r.GET("/test", func(c *gin.Context) { c.String(200, "ok") })

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 429 {
		t.Fatalf("status = %d, want 429", w.Code)
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(10, 20)
	// Should not panic on first stop
	rl.Stop()
	// Should not panic on second stop
	rl.Stop()
}
