package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// searchHTTP is a minimal HTTP client interface for dependency injection / testing.
type searchHTTP interface {
	Do(req *http.Request) (*http.Response, error)
}

// WebSearchTool performs web searches via a configurable search API backend.
// When search is disabled (default), InvokableRun returns a friendly error.
// Supported formats: serpapi, brave, bing.
type WebSearchTool struct {
	baseURL string
	apiKey  string
	engine  string
	format  string
	enabled bool
	client  searchHTTP
}

// NewWebSearchTool creates a WebSearchTool.
// Pass empty apiKey or enabled=false to disable search.
// format: "serpapi" (default), "brave", or "bing".
func NewWebSearchTool(baseURL, apiKey, engine, format string, enabled bool) *WebSearchTool {
	if format == "" {
		format = "serpapi"
	}
	if baseURL == "" {
		switch format {
		case "brave":
			baseURL = "https://api.search.brave.com/res/v1/web/search"
		case "bing":
			baseURL = "https://api.bing.microsoft.com/v7.0/search"
		default:
			baseURL = "https://serpapi.com/search"
		}
	}
	if engine == "" {
		engine = "google"
	}
	return &WebSearchTool{
		baseURL: baseURL,
		apiKey:  apiKey,
		engine:  engine,
		format:  format,
		enabled: enabled && apiKey != "",
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

// Info implements tool.BaseTool.
func (w *WebSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "web_search",
		Desc: "搜索互联网获取最新信息。当需要实时数据、新闻、或不在知识库中的内容时使用。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "搜索关键词",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun implements tool.InvokableTool.
func (w *WebSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
		return "", fmt.Errorf("web_search: invalid args: %w", err)
	}

	if !w.enabled {
		return "", fmt.Errorf(
			"web_search: 搜索功能未启用。请在 config.yaml 中设置 search.enabled=true 并配置 search.api_key",
		)
	}

	return w.search(ctx, args.Query)
}

func (w *WebSearchTool) search(ctx context.Context, query string) (string, error) {
	reqURL, err := url.Parse(w.baseURL)
	if err != nil {
		return "", fmt.Errorf("web_search: invalid base URL %q: %w", w.baseURL, err)
	}

	q := reqURL.Query()
	q.Set("q", query)

	switch w.format {
	case "brave":
		// Brave uses query params only for q, auth via header.
	case "bing":
		// Bing uses query params: q, mkt, etc. Auth via header.
		q.Set("mkt", "zh-CN")
	default:
		// SerpAPI: auth and engine via query params.
		q.Set("api_key", w.apiKey)
		q.Set("engine", w.engine)
	}
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("web_search: build request: %w", err)
	}

	// Per-format auth headers.
	switch w.format {
	case "brave":
		req.Header.Set("X-Subscription-Token", w.apiKey)
		req.Header.Set("Accept", "application/json")
	case "bing":
		req.Header.Set("Ocp-Apim-Subscription-Key", w.apiKey)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("web_search: 搜索请求失败，请检查网络或 API 配置: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("web_search: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("web_search: API 返回错误 (status=%d): %s", resp.StatusCode, string(body))
	}

	switch w.format {
	case "brave":
		return w.formatBraveResults(body)
	case "bing":
		return w.formatBingResults(body)
	default:
		return w.formatSerpapiResults(body)
	}
}

// formatSerpapiResults parses SerpAPI JSON response.
func (w *WebSearchTool) formatSerpapiResults(body []byte) (string, error) {
	var data struct {
		OrganicResults []struct {
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
			Link    string `json:"link"`
		} `json:"organic_results"`
		AnswerBox struct {
			Title   string `json:"title"`
			Answer  string `json:"answer"`
			Snippet string `json:"snippet"`
		} `json:"answer_box"`
		KnowledgeGraph struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"knowledge_graph"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("web_search: parse results: %w", err)
	}

	var b strings.Builder

	if data.AnswerBox.Answer != "" {
		b.WriteString("直接答案: ")
		b.WriteString(data.AnswerBox.Answer)
		b.WriteString("\n\n")
	} else if data.AnswerBox.Snippet != "" {
		b.WriteString("精选摘要: ")
		b.WriteString(data.AnswerBox.Snippet)
		b.WriteString("\n\n")
	}

	if data.KnowledgeGraph.Title != "" {
		b.WriteString("知识图谱: ")
		b.WriteString(data.KnowledgeGraph.Title)
		if data.KnowledgeGraph.Description != "" {
			b.WriteString(" — ")
			b.WriteString(data.KnowledgeGraph.Description)
		}
		b.WriteString("\n\n")
	}

	if len(data.OrganicResults) == 0 {
		b.WriteString("(无搜索结果)")
		return b.String(), nil
	}

	for i, r := range data.OrganicResults {
		if i >= 5 {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
		if r.Snippet != "" {
			b.WriteString("   ")
			b.WriteString(r.Snippet)
			b.WriteString("\n")
		}
		b.WriteString("   URL: ")
		b.WriteString(r.Link)
		b.WriteString("\n\n")
	}

	return b.String(), nil
}

// formatBraveResults parses Brave Search API response.
func (w *WebSearchTool) formatBraveResults(body []byte) (string, error) {
	var data struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("web_search: parse brave results: %w", err)
	}

	var b strings.Builder
	if len(data.Web.Results) == 0 {
		b.WriteString("(无搜索结果)")
		return b.String(), nil
	}

	for i, r := range data.Web.Results {
		if i >= 5 {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Title))
		if r.Description != "" {
			b.WriteString("   ")
			b.WriteString(r.Description)
			b.WriteString("\n")
		}
		b.WriteString("   URL: ")
		b.WriteString(r.URL)
		b.WriteString("\n\n")
	}

	return b.String(), nil
}

// formatBingResults parses Bing Search API response.
func (w *WebSearchTool) formatBingResults(body []byte) (string, error) {
	var data struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				Snippet string `json:"snippet"`
				URL     string `json:"url"`
			} `json:"value"`
		} `json:"webPages"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("web_search: parse bing results: %w", err)
	}

	var b strings.Builder
	if len(data.WebPages.Value) == 0 {
		b.WriteString("(无搜索结果)")
		return b.String(), nil
	}

	for i, r := range data.WebPages.Value {
		if i >= 5 {
			break
		}
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Name))
		if r.Snippet != "" {
			b.WriteString("   ")
			b.WriteString(r.Snippet)
			b.WriteString("\n")
		}
		b.WriteString("   URL: ")
		b.WriteString(r.URL)
		b.WriteString("\n\n")
	}

	return b.String(), nil
}

var _ tool.InvokableTool = (*WebSearchTool)(nil)
