package tool

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockHTTP is a test-only HTTP client that returns a canned response.
type mockHTTP struct {
	status int
	body   string
	err    error
}

func (m *mockHTTP) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.status,
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

func serpAPIResponse() string {
	r := map[string]any{
		"organic_results": []map[string]any{
			{"title": "Go 语言官网", "snippet": "Go是一门开源编程语言...", "link": "https://go.dev"},
			{"title": "Go Wiki", "snippet": "Go语言wiki页面", "link": "https://github.com/golang/go/wiki"},
		},
		"answer_box": map[string]any{
			"answer": "Go是由Google开发的开源编程语言。",
		},
	}
	b, _ := json.Marshal(r)
	return string(b)
}

func TestWebSearchTool_Info(t *testing.T) {
	tool := NewWebSearchTool("https://api.example.com", "key", "google", "", true)
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if info.Name != "web_search" {
		t.Errorf("Info().Name = %q, want %q", info.Name, "web_search")
	}
}

func TestWebSearchTool_InvokableRun_Disabled(t *testing.T) {
	tool := NewWebSearchTool("https://api.example.com", "", "google", "", false)
	args := `{"query":"test"}`
	// disabled 是软失败：返回提示文案而非 error（error 会以 NodeRunError 打断 Agent 图）
	result, err := tool.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("disabled should soft-fail with nil error, got: %v", err)
	}
	if !strings.Contains(result, "未启用") {
		t.Errorf("result should mention 未启用, got: %q", result)
	}
}

func TestWebSearchTool_InvokableRun_NoAPIKey(t *testing.T) {
	tool := NewWebSearchTool("https://api.example.com", "", "google", "", true)
	args := `{"query":"test"}`
	// 构造器将 enabled && apiKey!="" 归一化，无 key 等价 disabled → 软失败
	result, err := tool.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("no api_key should soft-fail with nil error, got: %v", err)
	}
	if !strings.Contains(result, "未启用") {
		t.Errorf("result should mention 未启用, got: %q", result)
	}
}

func TestWebSearchTool_InvokableRun_Success(t *testing.T) {
	tool := &WebSearchTool{
		baseURL: "https://api.example.com/search",
		apiKey:  "test-key",
		engine:  "google",
		format:  "serpapi",
		enabled: true,
		client:  &mockHTTP{status: 200, body: serpAPIResponse()},
	}

	args := `{"query":"golang"}`
	result, err := tool.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	if !strings.Contains(result, "Go语言") && !strings.Contains(result, "Go") {
		t.Errorf("result should contain Go-related content, got: %s", result)
	}
	if !strings.Contains(result, "go.dev") {
		t.Errorf("result should contain URL, got: %s", result)
	}
}

func TestWebSearchTool_InvokableRun_APIError(t *testing.T) {
	tool := &WebSearchTool{
		baseURL: "https://api.example.com/search",
		apiKey:  "test-key",
		engine:  "google",
		format:  "serpapi",
		enabled: true,
		client:  &mockHTTP{status: 401, body: "Unauthorized"},
	}

	args := `{"query":"test"}`
	_, err := tool.InvokableRun(context.Background(), args)
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should contain status code, got: %v", err)
	}
}

func TestWebSearchTool_InvokableRun_InvalidJSON(t *testing.T) {
	tool := NewWebSearchTool("https://api.example.com", "key", "google", "", true)
	_, err := tool.InvokableRun(context.Background(), `not-json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON args")
	}
	if !strings.Contains(err.Error(), "invalid args") {
		t.Errorf("error should mention invalid args, got: %v", err)
	}
}

func TestWebSearchTool_EmptyResults(t *testing.T) {
	emptyResp := `{"organic_results":[]}`
	tool := &WebSearchTool{
		baseURL: "https://api.example.com/search",
		apiKey:  "test-key",
		engine:  "google",
		format:  "serpapi",
		enabled: true,
		client:  &mockHTTP{status: 200, body: emptyResp},
	}

	result, err := tool.InvokableRun(context.Background(), `{"query":"dkfjdksfjkdlsfjls"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	if !strings.Contains(result, "无搜索结果") {
		t.Errorf("result should mention no results, got: %s", result)
	}
}
