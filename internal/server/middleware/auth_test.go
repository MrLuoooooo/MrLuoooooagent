package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

func init() { gin.SetMode(gin.TestMode) }

func TestAuth_DevMode_NoToken(t *testing.T) {
	cfg := &Config{APIKey: ""}
	router := gin.New()
	router.Use(Auth(cfg))
	router.GET("/test", func(c *gin.Context) {
		user, _ := c.Get(UserContextKey)
		c.JSON(200, gin.H{"user": user})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("dev mode should allow no token, got %d", w.Code)
	}
}

func TestAuth_DevMode_AnyToken(t *testing.T) {
	cfg := &Config{APIKey: ""}
	router := gin.New()
	router.Use(Auth(cfg))
	router.GET("/test", func(c *gin.Context) {
		user, _ := c.Get(UserContextKey)
		c.JSON(200, gin.H{"user": user})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("dev mode should accept any token, got %d", w.Code)
	}
}

func TestAuth_ProdMode_NoToken_Rejected(t *testing.T) {
	cfg := &Config{APIKey: "secret-key"}
	router := gin.New()
	router.Use(Auth(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	var env model.APIEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if env.Code != 401 {
		t.Errorf("response code = %d, want 401", env.Code)
	}
	if env.Message != "unauthorized" {
		t.Errorf("message = %q, want %q", env.Message, "unauthorized")
	}
}

func TestAuth_ProdMode_WrongToken_Rejected(t *testing.T) {
	cfg := &Config{APIKey: "secret-key"}
	router := gin.New()
	router.Use(Auth(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", w.Code)
	}
}

func TestAuth_ProdMode_ValidToken(t *testing.T) {
	cfg := &Config{APIKey: "secret-key"}
	router := gin.New()
	router.Use(Auth(cfg))
	router.GET("/test", func(c *gin.Context) {
		user, _ := c.Get(UserContextKey)
		c.JSON(200, gin.H{"user": user})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for valid token, got %d", w.Code)
	}
}

func TestAuth_ProdMode_NoBearerPrefix(t *testing.T) {
	cfg := &Config{APIKey: "secret-key"}
	router := gin.New()
	router.Use(Auth(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "secret-key")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing Bearer prefix, got %d", w.Code)
	}
}
