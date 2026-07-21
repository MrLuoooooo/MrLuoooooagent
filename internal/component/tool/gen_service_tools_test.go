package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
)

// newDemoSvc 创建测试用的 DemoService。
func newDemoSvc() *service.AgentToolDemoService {
	return service.NewAgentToolDemoService()
}

func TestGenEchoTool(t *testing.T) {
	tool := ProvideEchoTool(newDemoSvc())

	// 1. Info 返回正确的 schema
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "demo_echo" {
		t.Errorf("name = %q, want %q", info.Name, "demo_echo")
	}
	// 验证 ParamsOneOf 可序列化
	jsonSchema, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema: %v", err)
	}
	b, _ := json.Marshal(jsonSchema)
	if !strings.Contains(string(b), "message") {
		t.Error("JSON schema 应包含 'message' 参数")
	}
	t.Logf("Info ✓ name=%s desc=%s", info.Name, info.Desc)

	// 2. InvokableRun 正确执行
	result, err := tool.InvokableRun(context.Background(), `{"message":"hello"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, "hello") {
		t.Errorf("result = %q, want contain 'hello'", result)
	}

	// 3. 参数解析失败
	_, err = tool.InvokableRun(context.Background(), `not json`)
	if err == nil {
		t.Error("非法JSON参数应返回错误")
	}
	t.Logf("Echo ✓ result=%s", result)
}

func TestGenCountWordsTool(t *testing.T) {
	tool := ProvideCountWordsTool(newDemoSvc())

	info, _ := tool.Info(context.Background())
	if info.Name != "demo_count_words" {
		t.Errorf("name = %q", info.Name)
	}

	result, err := tool.InvokableRun(context.Background(), `{"text":"hello world go"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if result != "3" {
		t.Errorf("result = %q, want '3'", result)
	}
	t.Logf("CountWords ✓ result=%s", result)
}

func TestGenMatchTool(t *testing.T) {
	tool := ProvideMatchTool(newDemoSvc())

	info, _ := tool.Info(context.Background())
	jsonSchema, _ := info.ParamsOneOf.ToJSONSchema()
	b, _ := json.Marshal(jsonSchema)
	if !strings.Contains(string(b), "text") || !strings.Contains(string(b), "keyword") {
		t.Error("JSON schema 缺少 text/keyword 参数")
	}

	// 匹配正面
	result, err := tool.InvokableRun(context.Background(), `{"text":"hello world","keyword":"world"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if result != "true" {
		t.Errorf("result = %q, want 'true'", result)
	}

	// 不匹配
	result, err = tool.InvokableRun(context.Background(), `{"text":"hello world","keyword":"go"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if result != "false" {
		t.Errorf("result = %q, want 'false'", result)
	}
	t.Logf("Match ✓ matched=true unmatched=%s", result)
}

func TestGenSplitTool(t *testing.T) {
	tool := ProvideSplitTool(newDemoSvc())

	info, _ := tool.Info(context.Background())
	if info.Name != "demo_split" {
		t.Errorf("name = %q", info.Name)
	}

	result, err := tool.InvokableRun(context.Background(), `{"text":"a,b,c","sep":","}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	var arr []string
	if err := json.Unmarshal([]byte(result), &arr); err != nil {
		t.Fatalf("result is not valid JSON array: %v", err)
	}
	if len(arr) != 3 || arr[0] != "a" || arr[1] != "b" || arr[2] != "c" {
		t.Errorf("result = %v, want [a b c]", arr)
	}
	t.Logf("Split ✓ result=%v", arr)
}

func TestGenLogTool(t *testing.T) {
	tool := ProvideLogTool(newDemoSvc())

	info, _ := tool.Info(context.Background())
	if info.Name != "demo_log" {
		t.Errorf("name = %q", info.Name)
	}

	// error-only 返回的方法 → 成功返回 "{}"
	result, err := tool.InvokableRun(context.Background(), `{"message":"test log","level":"info"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if result != "{}" {
		t.Errorf("result = %q, want '{}'", result)
	}
	t.Logf("Log ✓ result=%s", result)
}

func TestGenToolRegistered(t *testing.T) {
	// 清空 registry
	globalRegistry.mu.Lock()
	globalRegistry.tools = nil
	globalRegistry.mu.Unlock()

	// 通过 Provider 注册
	ProvideEchoTool(newDemoSvc())
	ProvideCountWordsTool(newDemoSvc())
	ProvideMatchTool(newDemoSvc())
	ProvideSplitTool(newDemoSvc())
	ProvideLogTool(newDemoSvc())

	tools := RegisteredTools()
	if len(tools) != 5 {
		t.Fatalf("registered tools = %d, want 5", len(tools))
	}

	infos, err := ToolInfos(context.Background())
	if err != nil {
		t.Fatalf("ToolInfos: %v", err)
	}

	names := make(map[string]bool)
	for _, info := range infos {
		names[info.Name] = true
	}
	for _, want := range []string{"demo_echo", "demo_count_words", "demo_match", "demo_split", "demo_log"} {
		if !names[want] {
			t.Errorf("missing tool: %s", want)
		}
	}

	// 验证每个 ToolInfo 可序列化为 LLM function calling JSON
	for _, info := range infos {
		b, err := json.Marshal(info)
		if err != nil {
			t.Errorf("Failed to marshal tool %s: %v", info.Name, err)
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Errorf("Failed to unmarshal tool %s: %v", info.Name, err)
		}
		t.Logf("Tool[%s] JSON: %s", info.Name, string(b))
	}

	t.Logf("Register ✓ %d tools registered and JSON-serializable", len(infos))
}

func TestGenToolImplementsInterface(t *testing.T) {
	// 编译期接口检查（var _ InvokableTool = ...）已在生成代码中
	// 运行时验证 Info 不 panic
	tools := []Tool{
		ProvideEchoTool(newDemoSvc()),
		ProvideCountWordsTool(newDemoSvc()),
		ProvideMatchTool(newDemoSvc()),
		ProvideSplitTool(newDemoSvc()),
		ProvideLogTool(newDemoSvc()),
	}
	for _, tool := range tools {
		info, err := tool.Info(context.Background())
		if err != nil {
			t.Errorf("Info() error: %v", err)
		}
		if info.Name == "" {
			t.Error("Info().Name is empty")
		}
		if info.Desc == "" {
			t.Error("Info().Desc is empty")
		}
	}
	t.Logf("Interface ✓ %d tools implement Info + InvokableRun", len(tools))
}
