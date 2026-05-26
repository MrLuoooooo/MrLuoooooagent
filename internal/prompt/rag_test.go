package prompt

import (
	"context"
	"strings"
	"testing"
)

func TestNewRAGTemplate_ContainsSystemPrompt(t *testing.T) {
	tmpl := NewRAGTemplate()
	if tmpl == nil {
		t.Fatal("NewRAGTemplate() returned nil")
	}

	msgs, err := tmpl.Format(context.Background(), map[string]any{
		"context": "test-context",
		"query":   "hello",
	})
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("Format() returned empty messages")
	}

	content := msgs[0].Content
	if !strings.Contains(content, "Context:") {
		t.Error("system prompt missing Context label")
	}
	if !strings.Contains(content, "test-context") {
		t.Error("system prompt missing injected context value")
	}
	if !strings.Contains(content, "hello") {
		t.Error("system prompt missing injected query value")
	}
	if !strings.Contains(content, "Question:") {
		t.Error("system prompt missing Question label")
	}
}

func TestNewRAGTemplate_MissingVariable(t *testing.T) {
	tmpl := NewRAGTemplate()
	_, err := tmpl.Format(context.Background(), map[string]any{
		"context": "only context",
	})
	if err == nil {
		t.Error("Format() should return error for missing {query} variable")
	}
}
