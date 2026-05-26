package graph

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// toolCallBlockRe matches <tool_call>...</tool_call> XML blocks.
var toolCallBlockRe = regexp.MustCompile(`<tool_call>\s*(.*?)\s*</tool_call>`)

// jsonCodeBlockRe matches ```json ... ``` blocks that contain "tool" and "arguments" fields.
var jsonCodeBlockRe = regexp.MustCompile("```json\\s*\\n(.*?)```")

// parsePromptToolCalls attempts to extract tool calls from the model's text response.
// Returns nil if no tool call patterns are found. Used as a fallback when the model
// doesn't support native function calling.
func parsePromptToolCalls(content string) []schema.ToolCall {
	// Try XML format first.
	if tcs := parseXMLToolCalls(content); len(tcs) > 0 {
		return tcs
	}
	// Try JSON code block format.
	if tcs := parseJSONToolCalls(content); len(tcs) > 0 {
		return tcs
	}
	return nil
}

type xmlToolCall struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments"`
}

func parseXMLToolCalls(content string) []schema.ToolCall {
	matches := toolCallBlockRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	tcs := make([]schema.ToolCall, 0, len(matches))
	for i, m := range matches {
		if len(m) < 2 {
			continue
		}
		var tc xmlToolCall
		if err := json.Unmarshal([]byte(m[1]), &tc); err != nil {
			continue
		}
		argsJSON, _ := json.Marshal(tc.Arguments)
		tcs = append(tcs, schema.ToolCall{
			ID:   "prompt_tc_" + string(rune('0'+i)),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      tc.Name,
				Arguments: string(argsJSON),
			},
		})
	}
	return tcs
}

func parseJSONToolCalls(content string) []schema.ToolCall {
	matches := jsonCodeBlockRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	tcs := make([]schema.ToolCall, 0, len(matches))
	for i, m := range matches {
		if len(m) < 2 {
			continue
		}
		var parsed struct {
			Tool      string `json:"tool"`
			Name      string `json:"name"`
			Arguments any    `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(m[1]), &parsed); err != nil {
			continue
		}
		toolName := parsed.Tool
		if toolName == "" {
			toolName = parsed.Name
		}
		if toolName == "" {
			continue
		}
		argsJSON, _ := json.Marshal(parsed.Arguments)
		tcs = append(tcs, schema.ToolCall{
			ID:   "prompt_tc_" + string(rune('0'+i)),
			Type: "function",
			Function: schema.FunctionCall{
				Name:      toolName,
				Arguments: string(argsJSON),
			},
		})
	}
	return tcs
}

// stripToolCallBlocks removes <tool_call>...</tool_call> blocks from content
// so the user doesn't see raw tool call XML in the chat output.
func stripToolCallBlocks(content string) string {
	content = toolCallBlockRe.ReplaceAllString(content, "")
	content = jsonCodeBlockRe.ReplaceAllString(content, "")
	return strings.TrimSpace(content)
}

// buildPromptToolsSection generates a text description of all available tools
// for inclusion in the system prompt. Used when the model doesn't support
// native function calling.
func buildPromptToolsSection(infos []*schema.ToolInfo) string {
	if len(infos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## 可用工具\n\n")
	b.WriteString("你可以使用以下工具。要调用工具，必须在回复中输出如下格式的 XML：\n\n")
	b.WriteString("<tool_call>\n{\"name\": \"工具名\", \"arguments\": {\"参数名\": \"参数值\"}}\n</tool_call>\n\n")
	b.WriteString("你可以在同一条回复中调用多个工具（多个 <tool_call> 块）。")
	b.WriteString("工具执行结果会自动返回给你，你再继续思考。")
	b.WriteString("如果你不需要使用工具，直接文字回复即可。\n\n")

	for _, t := range infos {
		b.WriteString("### ")
		b.WriteString(t.Name)
		b.WriteString("\n")
		b.WriteString(t.Desc)
		b.WriteString("\n")
		if t.ParamsOneOf != nil {
			sc, err := t.ParamsOneOf.ToJSONSchema()
			if err != nil || sc == nil || sc.Properties == nil {
				b.WriteString("\n")
				continue
			}
			b.WriteString("参数：\n")
			for pair := sc.Properties.Oldest(); pair != nil; pair = pair.Next() {
				pname := pair.Key
				pval := pair.Value
				b.WriteString("- ")
				b.WriteString(pname)
				b.WriteString(" (")
				b.WriteString(pval.Type)
				b.WriteString("): ")
				b.WriteString(pval.Description)
				for _, r := range sc.Required {
					if r == pname {
						b.WriteString(" [必填]")
						break
					}
				}
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
