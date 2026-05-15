package handler

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestPrependHistory_Basic(t *testing.T) {
	history := []*schema.Message{
		{Role: schema.User, Content: "什么是Go?"},
		{Role: schema.Assistant, Content: "Go是Google开发的开源编程语言。"},
	}
	result := prependHistory("有什么优势?", history)

	if !strings.Contains(result, "什么是Go?") {
		t.Errorf("history should contain user message, got: %s", result)
	}
	if !strings.Contains(result, "Go是Google") {
		t.Errorf("history should contain assistant message, got: %s", result)
	}
	if !strings.Contains(result, "有什么优势?") {
		t.Errorf("should end with user question, got: %s", result)
	}
}

func TestPrependHistory_Empty(t *testing.T) {
	result := prependHistory("hello", nil)
	if !strings.Contains(result, "hello") {
		t.Errorf("should contain question with no history, got: %s", result)
	}
}

func TestPrependHistory_WithToolCalls(t *testing.T) {
	history := []*schema.Message{
		{Role: schema.User, Content: "搜索Go语言"},
		{Role: schema.Assistant, Content: "", ToolCalls: []schema.ToolCall{
			{ID: "1", Function: schema.FunctionCall{Name: "web_search", Arguments: `{"query":"Go语言"}`}},
		}},
		{Role: schema.Tool, Content: "搜索结果: Go语言是由Google开发的..."},
		{Role: schema.Assistant, Content: "搜索完成。Go语言是一门编译型语言。"},
	}
	result := prependHistory("介绍一下", history)

	if !strings.Contains(result, "web_search") {
		t.Errorf("history should contain tool call name, got: %s", result)
	}
	if !strings.Contains(result, "调用工具") {
		t.Errorf("history should indicate tool call, got: %s", result)
	}
	if !strings.Contains(result, "tool_result") {
		t.Errorf("history should contain tool result, got: %s", result)
	}
	if !strings.Contains(result, "搜索完成") {
		t.Errorf("history should contain final assistant reply, got: %s", result)
	}
}

func TestPrependHistory_MultipleToolCalls(t *testing.T) {
	history := []*schema.Message{
		{Role: schema.User, Content: "现在几点了？"},
		{Role: schema.Assistant, Content: "", ToolCalls: []schema.ToolCall{
			{ID: "tc1", Function: schema.FunctionCall{Name: "get_current_datetime", Arguments: `{"format":"rfc3339"}`}},
		}},
		{Role: schema.Tool, Content: "2026-05-13T10:30:00Z"},
		{Role: schema.Assistant, Content: "现在是2026年5月13日10:30。"},
	}
	result := prependHistory("再查一下天气", history)

	if !strings.Contains(result, "get_current_datetime") {
		t.Errorf("should contain datetime tool call")
	}
	if !strings.Contains(result, "2026-05-13T10:30:00Z") {
		t.Errorf("should contain tool result timestamp")
	}
}

func TestPrependHistory_NoContentAssistant(t *testing.T) {
	// When assistant only emits tool calls and no text content.
	history := []*schema.Message{
		{Role: schema.User, Content: "hi"},
		{Role: schema.Assistant, Content: "", ToolCalls: []schema.ToolCall{
			{ID: "t1", Function: schema.FunctionCall{Name: "web_search", Arguments: `{"query":"news"}`}},
		}},
	}
	result := prependHistory("next question", history)

	// Should still include the tool call but not duplicate empty assistant content.
	if !strings.Contains(result, "web_search") {
		t.Errorf("should include tool call even when content is empty")
	}
	// Should NOT have "assistant: \n" (empty content followed by newline without text)
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if line == "assistant: " {
			t.Errorf("should not have empty assistant content line")
		}
	}
}
