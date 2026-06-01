package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"go.uber.org/zap"
)

func resetWorkspaceEnv() {
	os.Unsetenv("HOST_MNT_PREFIX")
}

func TestWorkspaceGetCurrent(t *testing.T) {
	resetWorkspaceEnv()
	gin.SetMode(gin.TestMode)
	tmpDir := t.TempDir()
	h := &WorkspaceHandler{currentDir: tmpDir, logger: zap.NewNop()}
	r := gin.New()
	r.GET("/workspace", h.GetCurrent)

	req := httptest.NewRequest("GET", "/workspace", nil)
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

func TestWorkspaceSetCurrent(t *testing.T) {
	resetWorkspaceEnv()
	gin.SetMode(gin.TestMode)
	h := &WorkspaceHandler{currentDir: t.TempDir(), logger: zap.NewNop()}
	r := gin.New()
	r.POST("/workspace", h.SetCurrent)

	tmpDir := filepath.ToSlash(t.TempDir())
	body := bytes.NewReader([]byte(`{"path":"` + tmpDir + `"}`))
	req := httptest.NewRequest("POST", "/workspace", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
}

func TestWorkspaceSetCurrent_MissingPath(t *testing.T) {
	resetWorkspaceEnv()
	gin.SetMode(gin.TestMode)
	h := &WorkspaceHandler{currentDir: t.TempDir(), logger: zap.NewNop()}
	r := gin.New()
	r.POST("/workspace", h.SetCurrent)

	body := bytes.NewReader([]byte(`{}`))
	req := httptest.NewRequest("POST", "/workspace", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestBuildTree_Success(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644)

	h := &WorkspaceHandler{logger: zap.NewNop()}
	tree, err := h.buildTree(tmpDir, 3)
	if err != nil {
		t.Fatalf("buildTree: %v", err)
	}
	if !tree.IsDir {
		t.Error("expected root to be a directory")
	}
	if len(tree.Children) == 0 {
		t.Error("expected children in temp dir")
	}
	foundTestFile := false
	for _, c := range tree.Children {
		if c.Name == "test.txt" {
			foundTestFile = true
			break
		}
	}
	if !foundTestFile {
		t.Error("test.txt not found in children")
	}
}

func TestBuildTree_InvalidPath(t *testing.T) {
	h := &WorkspaceHandler{logger: zap.NewNop()}
	_, err := h.buildTree(filepath.Join(t.TempDir(), "nonexistent"), 3)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestBuildTree_FileNode(t *testing.T) {
	tmpDir := t.TempDir()
	fpath := filepath.Join(tmpDir, "myfile.txt")
	os.WriteFile(fpath, []byte("content"), 0644)

	h := &WorkspaceHandler{logger: zap.NewNop()}
	tree, err := h.buildTree(fpath, 3)
	if err != nil {
		t.Fatalf("buildTree: %v", err)
	}
	if tree.IsDir {
		t.Error("expected a file node")
	}
	if tree.Size <= 0 {
		t.Error("expected positive size for file")
	}
}

func TestBuildTree_DepthLimit(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, "a", "b"), 0755)

	h := &WorkspaceHandler{logger: zap.NewNop()}
	tree, err := h.buildTree(tmpDir, 1)
	if err != nil {
		t.Fatalf("buildTree: %v", err)
	}
	if len(tree.Children) == 0 {
		t.Error("expected at least one child")
	}
}

func TestBuildTree_SkipsHidden(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(filepath.Join(tmpDir, ".hidden"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "normal"), 0755)

	h := &WorkspaceHandler{logger: zap.NewNop()}
	tree, err := h.buildTree(tmpDir, 3)
	if err != nil {
		t.Fatalf("buildTree: %v", err)
	}
	for _, c := range tree.Children {
		if strings.HasPrefix(c.Name, ".") {
			t.Errorf("hidden dir %q should be skipped", c.Name)
		}
	}
}

func TestWorkspaceListDrives(t *testing.T) {
	resetWorkspaceEnv()
	gin.SetMode(gin.TestMode)
	h := &WorkspaceHandler{currentDir: t.TempDir(), logger: zap.NewNop()}
	r := gin.New()
	r.GET("/workspace/drives", h.ListDrives)

	req := httptest.NewRequest("GET", "/workspace/drives", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestToWin(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`D:\path`, `D:\path`},
		{"/D/project", "D:\\project"},
		{"/D/project/sub", "D:\\project/sub"},
		{"/E/data", "/E/data"},
	}
	for _, tt := range tests {
		got := toWin(tt.input)
		if got != tt.want {
			t.Errorf("toWin(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToContainer(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`D:\project`, "/D/project"},
		{`D:\project\sub`, "/D/project\\sub"},
		{"/D/project", "/D/project"},
	}
	for _, tt := range tests {
		got := toContainer(tt.input)
		if got != tt.want {
			t.Errorf("toContainer(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestToWinWithHostMntPrefix(t *testing.T) {
	os.Setenv("HOST_MNT_PREFIX", "/mnt/host")
	defer os.Unsetenv("HOST_MNT_PREFIX")
	if got := toWin("/mnt/host/D/project"); got != "D:\\project" {
		t.Errorf("got %q", got)
	}
}

func TestToContainerWithHostMntPrefix(t *testing.T) {
	os.Setenv("HOST_MNT_PREFIX", "/mnt/host")
	defer os.Unsetenv("HOST_MNT_PREFIX")
	if got := toContainer(`D:\project`); got != "/mnt/host/d/project" {
		t.Errorf("got %q", got)
	}
}
