package prompt

import (
	"strings"
	"testing"
)

func TestSystemRAG_HasVariables(t *testing.T) {
	if !strings.Contains(systemRAG, "{context}") {
		t.Error("systemRAG should contain {context} placeholder")
	}
	if !strings.Contains(systemRAG, "{query}") {
		t.Error("systemRAG should contain {query} placeholder")
	}
}

func TestSystemRAG_NonEmpty(t *testing.T) {
	if len(systemRAG) == 0 {
		t.Error("systemRAG should not be empty")
	}
}

func TestSystemRAG_Structure(t *testing.T) {
	lines := strings.Split(systemRAG, "\n")
	if len(lines) < 5 {
		t.Errorf("expected multiple lines, got %d", len(lines))
	}
	// Should have context and question sections
	foundContext := false
	foundQuestion := false
	for _, line := range lines {
		if strings.Contains(line, "Context:") {
			foundContext = true
		}
		if strings.Contains(line, "Question:") {
			foundQuestion = true
		}
	}
	if !foundContext {
		t.Error("should have 'Context:' section")
	}
	if !foundQuestion {
		t.Error("should have 'Question:' section")
	}
}
