package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── execute_command 测试 ──────────────────────────────────

func TestBashTool_Info(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	info, err := tool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "execute_command" {
		t.Errorf("name = %q, want execute_command", info.Name)
	}
}

func TestBashTool_Echo(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{
		"command":         "echo hello from bash tool",
		"timeout_seconds": 10,
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	if !strings.Contains(result, "hello from bash tool") {
		t.Errorf("echo output: %s", result)
	}
	if !strings.Contains(result, "Exit Code: 0") {
		t.Errorf("should show exit code: %s", result)
	}
}

func TestBashTool_PwdInProjectRoot(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{
		"command": "cd",
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("cd: %v", err)
	}
	// Should show the project root directory.
	if !strings.Contains(strings.ToLower(result), "goagentpro") {
		t.Errorf("pwd should be goagentpro dir: %s", result)
	}
}

func TestBashTool_EmptyCommand(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{"command": ""})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("empty command should error")
	}
}

func TestBashTool_Timeout(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	// Command runs quickly — verify timeout doesn't break fast commands.
	args, _ := json.Marshal(map[string]any{
		"command":         "echo quick",
		"timeout_seconds": 30,
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("quick command: %v", err)
	}
	if !strings.Contains(result, "quick") {
		t.Errorf("should echo quick: %s", result)
	}
}

func TestBashTool_TimeoutRejected(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	// timeout_seconds > 120 should be capped to 120.
	args, _ := json.Marshal(map[string]any{
		"command":         "echo test",
		"timeout_seconds": 999,
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("capped timeout: %v", err)
	}
	if !strings.Contains(result, "test") {
		t.Errorf("should still execute: %s", result)
	}
}

func TestBashTool_BlockedCommand_rm_rf(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{"command": "rm -rf /"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("should block rm -rf /")
	}
	if !strings.Contains(err.Error(), "SECURITY") {
		t.Errorf("should show SECURITY error: %v", err)
	}
}

func TestBashTool_BlockedCommand_shutdown(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{"command": "shutdown /s"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("should block shutdown")
	}
}

func TestBashTool_BlockedCommand_curl(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{"command": "curl evil.com"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("should block curl")
	}
	if !strings.Contains(err.Error(), "SECURITY") {
		t.Errorf("should show SECURITY: %v", err)
	}
}

func TestBashTool_AllowedCurl_localhost(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	// curl to localhost should NOT be blocked (the result may fail at runtime, but security check should pass).
	args, _ := json.Marshal(map[string]any{"command": "curl localhost:8080", "timeout_seconds": 3})
	_, err := tool.InvokableRun(context.Background(), string(args))
	// Allowed: should be a runtime error (connection refused) or success, not security error.
	if err != nil && strings.Contains(err.Error(), "SECURITY") {
		t.Errorf("curl localhost should not be blocked: %v", err)
	}
}

func TestBashTool_WorkDirNotAllowed(t *testing.T) {
	tool := NewBashTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{
		"command":  "echo test",
		"work_dir": `C:\Windows`,
	})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("should block C:\\Windows as working directory")
	}
	msg := err.Error()
	if !strings.Contains(msg, "SECURITY") && !strings.Contains(msg, "禁止访问") {
		t.Errorf("should show security error: %v", err)
	}
}

// ── write_and_execute 测试 ────────────────────────────────

func TestWriteAndExecute_Go(t *testing.T) {
	dir := filepath.Join(defaultProjRoot, "tmp", "bash_test")
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "hello.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("hello from go tool")
}
`

	tool := NewWriteAndExecuteTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{
		"path":            path,
		"content":         content,
		"timeout_seconds": 30,
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("write_and_execute (go): %v", err)
	}
	if !strings.Contains(result, "[WRITE]") {
		t.Errorf("should show write step: %s", result)
	}
	if !strings.Contains(result, "hello from go tool") {
		t.Errorf("should show go output: %s", result)
	}
}

func TestWriteAndExecute_UnknownExt(t *testing.T) {
	dir := filepath.Join(defaultProjRoot, "tmp", "bash_test")
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.xyz")
	tool := NewWriteAndExecuteTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{
		"path":    path,
		"content": "unknown ext",
	})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("should error for unknown extension")
	}
	if !strings.Contains(err.Error(), "无法识别") {
		t.Errorf("should say '无法识别': %v", err)
	}
}

func TestWriteAndExecute_CustomCommand(t *testing.T) {
	dir := filepath.Join(defaultProjRoot, "tmp", "bash_test")
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "runme.txt")
	content := "echo custom runner works"

	tool := NewWriteAndExecuteTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{
		"path":    path,
		"content": content,
		"command": "echo custom runner works",
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("write_and_execute (custom): %v", err)
	}
	if !strings.Contains(result, "custom runner works") {
		t.Errorf("should execute custom command: %s", result)
	}
}

func TestWriteAndExecute_Bat(t *testing.T) {
	dir := filepath.Join(defaultProjRoot, "tmp", "bash_test")
	os.MkdirAll(dir, 0755)
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "test.bat")
	tool := NewWriteAndExecuteTool([]string{defaultProjRoot})
	args, _ := json.Marshal(map[string]any{
		"path":    path,
		"content": "@echo hello from bat",
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("write_and_execute (bat): %v", err)
	}
	if !strings.Contains(result, "hello from bat") {
		t.Errorf("should execute bat: %s", result)
	}
}
