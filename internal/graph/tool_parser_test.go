package graph

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestParseXMLToolCalls_Single(t *testing.T) {
	content := `Let me list the directory.

<tool_call>
{"name": "list_directory", "arguments": {"path": "D:\\"}}
</tool_call>`

	tcs := parsePromptToolCalls(content)
	if len(tcs) != 1 {
		t.Fatalf("len = %d, want 1", len(tcs))
	}
	if tcs[0].Function.Name != "list_directory" {
		t.Errorf("name = %q, want list_directory", tcs[0].Function.Name)
	}
	if tcs[0].Function.Arguments != `{"path":"D:\\"}` {
		t.Errorf("args = %q", tcs[0].Function.Arguments)
	}
}

func TestParseXMLToolCalls_Multiple(t *testing.T) {
	content := `<tool_call>
{"name": "read_file", "arguments": {"path": "README.md"}}
</tool_call>

<tool_call>
{"name": "get_current_datetime", "arguments": {}}
</tool_call>`

	tcs := parsePromptToolCalls(content)
	if len(tcs) != 2 {
		t.Fatalf("len = %d, want 2", len(tcs))
	}
	if tcs[0].Function.Name != "read_file" {
		t.Errorf("tc[0].name = %q", tcs[0].Function.Name)
	}
	if tcs[1].Function.Name != "get_current_datetime" {
		t.Errorf("tc[1].name = %q", tcs[1].Function.Name)
	}
}

func TestParseXMLToolCalls_None(t *testing.T) {
	content := "I can help you with that. What would you like to know?"
	tcs := parsePromptToolCalls(content)
	if len(tcs) != 0 {
		t.Errorf("len = %d, want 0", len(tcs))
	}
}

func TestBuildPromptToolsSection(t *testing.T) {
	infos := []*schema.ToolInfo{
		{
			Name: "read_file",
			Desc: "Read a file",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"path":     {Type: schema.String, Desc: "File path", Required: true},
				"max_size": {Type: schema.Integer, Desc: "Max bytes", Required: false},
			}),
		},
	}
	result := buildPromptToolsSection(infos)
	if !strings.Contains(result, "read_file") {
		t.Error("should contain tool name")
	}
	if !strings.Contains(result, "Read a file") {
		t.Error("should contain tool description")
	}
	if !strings.Contains(result, "path") {
		t.Error("should contain parameter name")
	}
	if !strings.Contains(result, "必填") {
		t.Error("should mark required params")
	}
	if !strings.Contains(result, "max_size") {
		t.Error("should contain optional param")
	}
	if !strings.Contains(result, "<tool_call>") {
		t.Error("should contain tool call format instruction")
	}
}

func TestBuildPromptToolsSection_Empty(t *testing.T) {
	result := buildPromptToolsSection(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
	result = buildPromptToolsSection([]*schema.ToolInfo{})
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
