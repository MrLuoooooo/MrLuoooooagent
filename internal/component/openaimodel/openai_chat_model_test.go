package openaimodel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestBindTools_StoresTools(t *testing.T) {
	m := &OpenAIChatModel{}
	tools := []*schema.ToolInfo{
		{
			Name: "search",
			Desc: "Search the web",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"query": {Type: schema.String, Desc: "Search query", Required: true},
			}),
		},
		{
			Name: "get_time",
			Desc: "Get current time",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"tz": {Type: schema.String, Desc: "Timezone", Required: false},
			}),
		},
	}

	if err := m.BindTools(tools); err != nil {
		t.Fatalf("BindTools: %v", err)
	}
	if len(m.tools) != 2 {
		t.Fatalf("tools len = %d, want 2", len(m.tools))
	}

	for _, tool := range m.tools {
		if tool.Type != "function" {
			t.Errorf("tool.Type = %q, want function", tool.Type)
		}
		if tool.Function.Name == "" {
			t.Error("tool.Function.Name should not be empty")
		}
		if tool.Function.Description == "" {
			t.Error("tool.Function.Description should not be empty")
		}
		// Verify that ParamsOneOf was converted to JSON schema.
		if tool.Function.Parameters == nil {
			t.Errorf("tool %s: Parameters should not be nil", tool.Function.Name)
		}
	}
}

func TestBindTools_EmptyTools(t *testing.T) {
	m := &OpenAIChatModel{}
	if err := m.BindTools(nil); err != nil {
		t.Fatalf("BindTools nil: %v", err)
	}
	if len(m.tools) != 0 {
		t.Errorf("tools len = %d, want 0", len(m.tools))
	}
}

func TestConvertMessages(t *testing.T) {
	input := []*schema.Message{
		{Role: schema.System, Content: "You are a helpful assistant."},
		{Role: schema.User, Content: "Hello"},
	}
	result := convertMessages(input)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if result[0].Role != "system" {
		t.Errorf("msg[0].Role = %q, want system", result[0].Role)
	}
	if result[0].Content != "You are a helpful assistant." {
		t.Errorf("msg[0].Content mismatch")
	}
	if result[1].Role != "user" {
		t.Errorf("msg[1].Role = %q, want user", result[1].Role)
	}
}

func TestConvertMessages_WithToolCalls(t *testing.T) {
	input := []*schema.Message{
		{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: schema.FunctionCall{
						Name:      "search",
						Arguments: `{"query":"hello"}`,
					},
				},
			},
		},
		{
			Role:       schema.Tool,
			Content:    "result from search",
			ToolCallID: "call_1",
		},
	}
	result := convertMessages(input)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	if len(result[0].ToolCalls) != 1 {
		t.Fatalf("msg[0].ToolCalls len = %d, want 1", len(result[0].ToolCalls))
	}
	tc := result[0].ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("tc.ID = %q, want call_1", tc.ID)
	}
	if tc.Function.Name != "search" {
		t.Errorf("tc.Function.Name = %q, want search", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"query":"hello"}` {
		t.Errorf("tc.Function.Arguments = %q", tc.Function.Arguments)
	}

	if result[1].ToolCallID != "call_1" {
		t.Errorf("msg[1].ToolCallID = %q, want call_1", result[1].ToolCallID)
	}
	if result[1].Content != "result from search" {
		t.Errorf("msg[1].Content mismatch")
	}
}

func TestConvertToMessage(t *testing.T) {
	om := openAIMessage{
		Role:    "assistant",
		Content: "I found results",
		ToolCalls: []openAIToolCall{
			{
				ID:   "tc_1",
				Type: "function",
				Function: openAIFunctionCall{
					Name:      "lookup",
					Arguments: `{"key":"val"}`,
				},
			},
		},
	}
	msg := convertToMessage(om)
	if msg.Role != schema.Assistant {
		t.Errorf("role = %v, want Assistant", msg.Role)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("ToolCalls len = %d, want 1", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "tc_1" {
		t.Errorf("ToolCall ID = %q", msg.ToolCalls[0].ID)
	}
}

func TestConvertToMessage_WithToolCallID(t *testing.T) {
	om := openAIMessage{
		Role:       "tool",
		Content:    "result",
		ToolCallID: "call_xyz",
	}
	msg := convertToMessage(om)
	if msg.ToolCallID != "call_xyz" {
		t.Errorf("ToolCallID = %q, want call_xyz", msg.ToolCallID)
	}
}

func TestGenerate_RequestFormat(t *testing.T) {
	var capturedReq openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&capturedReq); err != nil {
			t.Errorf("decode: %v", err)
		}
		if r.Header.Get("Authorization") != "Bearer my-api-key" {
			t.Errorf("bad auth: %s", r.Header.Get("Authorization"))
		}
		resp := openAIChatResponse{
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIMessage{
						Role:    "assistant",
						Content: "Hello, how can I help?",
					},
					FinishReason: "stop",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	m := &OpenAIChatModel{
		apiKey:  "my-api-key",
		model:   "gpt-4-test",
		baseURL: srv.URL,
		client:  srv.Client(),
	}
	input := []*schema.Message{
		{Role: schema.System, Content: "Be helpful."},
		{Role: schema.User, Content: "Hi"},
	}
	result, err := m.Generate(context.Background(), input)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Content != "Hello, how can I help?" {
		t.Errorf("content = %q", result.Content)
	}

	if capturedReq.Model != "gpt-4-test" {
		t.Errorf("req.Model = %q, want gpt-4-test", capturedReq.Model)
	}
	if capturedReq.Stream != false {
		t.Error("req.Stream should be false")
	}
	if len(capturedReq.Messages) != 2 {
		t.Errorf("req.Messages len = %d, want 2", len(capturedReq.Messages))
	}
}

func TestGenerate_WithOptions(t *testing.T) {
	var capturedReq openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&capturedReq)
		resp := openAIChatResponse{
			Choices: []openAIChoice{
				{Index: 0, Message: openAIMessage{Role: "assistant", Content: "ok"}, FinishReason: "stop"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	m := &OpenAIChatModel{
		apiKey:  "key",
		model:   "test",
		baseURL: srv.URL,
		client:  srv.Client(),
	}
	temp := float32(0.7)
	tokens := 100
	input := []*schema.Message{{Role: schema.User, Content: "Hi"}}

	_, err := m.Generate(context.Background(), input, model.WithTemperature(temp), model.WithMaxTokens(tokens))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if capturedReq.Temperature == nil {
		t.Fatal("temperature is nil")
	}
	got := *capturedReq.Temperature
	if got < 0.69 || got > 0.71 {
		t.Errorf("temperature = %v, want ~0.7", got)
	}
	if capturedReq.MaxTokens == nil || *capturedReq.MaxTokens != 100 {
		t.Errorf("maxTokens = %v, want 100", capturedReq.MaxTokens)
	}
}

func TestGenerate_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "server error"}`))
	}))
	defer srv.Close()

	m := &OpenAIChatModel{
		apiKey:  "key",
		model:   "test",
		baseURL: srv.URL,
		client:  srv.Client(),
	}
	_, err := m.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "OpenAI error (500)") {
		t.Errorf("error should contain status code: %v", err)
	}
}

func TestGenerate_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(openAIChatResponse{Choices: nil})
	}))
	defer srv.Close()

	m := &OpenAIChatModel{
		apiKey:  "key",
		model:   "test",
		baseURL: srv.URL,
		client:  srv.Client(),
	}
	_, err := m.Generate(context.Background(), []*schema.Message{{Role: schema.User, Content: "Hi"}})
	if err == nil {
		t.Fatal("expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error should mention 'no choices': %v", err)
	}
}

func TestParseOptions(t *testing.T) {
	temp := float32(0.5)
	tokens := 200
	topP := float32(0.9)
	opts := []model.Option{
		model.WithTemperature(temp),
		model.WithMaxTokens(tokens),
		model.WithTopP(topP),
	}
	o := parseOptions(opts)
	if o.temperature == nil {
		t.Fatal("temperature is nil")
	}
	got := *o.temperature
	if got < 0.49 || got > 0.51 {
		t.Errorf("temperature = %v, want ~0.5", got)
	}
	if o.maxTokens == nil || *o.maxTokens != 200 {
		t.Errorf("maxTokens = %v, want 200", o.maxTokens)
	}
	if o.topP == nil {
		t.Fatal("topP is nil")
	}
	gotP := *o.topP
	if gotP < 0.89 || gotP > 0.91 {
		t.Errorf("topP = %v, want ~0.9", gotP)
	}

	// Empty options returns zero-value chatOptions (all nil).
	o2 := parseOptions(nil)
	if o2.temperature != nil || o2.maxTokens != nil || o2.topP != nil {
		t.Error("empty options should produce nil fields")
	}
}
