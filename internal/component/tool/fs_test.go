package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempDir creates a temp directory under D:\goagentpro\tmp\ for testing file tools.
func tempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(defaultProjRoot, "tmp", t.Name())
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// ── write_file 测试 ───────────────────────────────────────

func TestWriteFile_CreateAndOverwrite(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.txt")

	tool := &WriteFileTool{}
	content := "hello world\n"
	args, _ := json.Marshal(map[string]string{"path": path, "content": content})

	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("write_file: %v", err)
	}
	if !strings.Contains(result, "[OK]") {
		t.Errorf("expected [OK], got: %s", result)
	}

	// Verify file exists and content matches.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}

	// Overwrite.
	content2 := "new content\n"
	args2, _ := json.Marshal(map[string]string{"path": path, "content": content2})
	result2, err := tool.InvokableRun(nil, string(args2))
	if err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	if !strings.Contains(result2, "[OK]") {
		t.Errorf("overwrite should succeed: %s", result2)
	}
}

func TestWriteFile_AutoCreateDir(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "sub", "deep", "file.txt")

	tool := &WriteFileTool{}
	args, _ := json.Marshal(map[string]string{"path": path, "content": "auto-created dir"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("auto-create dir: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}

func TestWriteFile_BlockBinaryExt(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.exe")

	tool := &WriteFileTool{}
	args, _ := json.Marshal(map[string]string{"path": path, "content": "fake"})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("should block .exe overwrite")
	}
	if !strings.Contains(err.Error(), "安全限制") || !strings.Contains(err.Error(), "可执行文件") {
		t.Errorf("error message wrong: %v", err)
	}
}

// ── read_file 测试 ────────────────────────────────────────

func TestReadFile_Success(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "readme.md")
	content := "# Hello\nThis is a test file.\n"
	os.WriteFile(path, []byte(content), 0644)

	tool := &ReadFileTool{}
	args, _ := json.Marshal(map[string]string{"path": path})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if result != content {
		t.Errorf("content = %q, want %q", result, content)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "nonexistent.txt")

	tool := &ReadFileTool{}
	args, _ := json.Marshal(map[string]string{"path": path})
	// Design: a missing file returns a self-healing hint (not an error) so the
	// LLM can recover by calling write_file itself.
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("missing file should return hint, not error: %v", err)
	}
	if !strings.Contains(result, "File not found") || !strings.Contains(result, "write_file") {
		t.Errorf("expected self-healing hint, got: %s", result)
	}
}

func TestReadFile_MaxSize(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "big.txt")
	content := strings.Repeat("A", 10000)
	os.WriteFile(path, []byte(content), 0644)

	tool := &ReadFileTool{}
	args, _ := json.Marshal(map[string]any{"path": path, "max_size": 100})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("read_file: %v", err)
	}
	if !strings.Contains(result, "[WARNING]") {
		t.Errorf("expected warning for large file, got: %s", result[:min(len(result), 200)])
	}
	if len(result) < 100 {
		t.Errorf("result too short for max_size=100")
	}
}

func TestReadFile_SensitiveMasked(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, ".env")
	os.WriteFile(path, []byte("MYSQL_PASSWORD=hunter2secret\nLOG_LEVEL=info"), 0644)

	tool := &ReadFileTool{}
	args, _ := json.Marshal(map[string]string{"path": path})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("sensitive file should be readable with masking, got: %v", err)
	}
	if !strings.Contains(result, "已脱敏") {
		t.Errorf("result should note masking, got: %q", result)
	}
	if strings.Contains(result, "hunter2secret") {
		t.Errorf("secret value leaked: %q", result)
	}
	if !strings.Contains(result, "LOG_LEVEL=info") {
		t.Errorf("non-sensitive lines should stay: %q", result)
	}
}

// ── edit_file 测试 ────────────────────────────────────────

func TestEditFile_Success(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "code.go")
	original := "package main\n\nfunc add(a, b int) int {\n\treturn a + b\n}\n"
	os.WriteFile(path, []byte(original), 0644)

	tool := &EditFileTool{}
	args, _ := json.Marshal(map[string]string{
		"path":       path,
		"old_string": "func add(a, b int) int {\n\treturn a + b\n}",
		"new_string": "func add(a, b int) int {\n\treturn a + b + 1\n}",
	})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("edit_file: %v", err)
	}
	if !strings.Contains(result, "[OK]") {
		t.Errorf("expected [OK], got: %s", result)
	}

	// Verify replacement.
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "return a + b + 1") {
		t.Errorf("edit not applied: %s", string(data))
	}
}

func TestEditFile_NoMatch(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello world"), 0644)

	tool := &EditFileTool{}
	args, _ := json.Marshal(map[string]string{
		"path":       path,
		"old_string": "nonexistent",
		"new_string": "replacement",
	})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("expected error for unmatched old_string")
	}
	if !strings.Contains(err.Error(), "找不到匹配") {
		t.Errorf("error wrong: %v", err)
	}
}

func TestEditFile_MultipleMatches(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello hello hello"), 0644)

	tool := &EditFileTool{}
	args, _ := json.Marshal(map[string]string{
		"path":       path,
		"old_string": "hello",
		"new_string": "hi",
	})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("expected error for multiple matches")
	}
	if !strings.Contains(err.Error(), "处匹配") {
		t.Errorf("error wrong: %v", err)
	}
}

// ── delete_file 测试 ──────────────────────────────────────

func TestDeleteFile_Success(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "todelete.txt")
	os.WriteFile(path, []byte("delete me"), 0644)

	tool := &DeleteFileTool{}
	args, _ := json.Marshal(map[string]string{"path": path})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("delete_file: %v", err)
	}
	if !strings.Contains(result, "已删除") {
		t.Errorf("expected '已删除', got: %s", result)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should be gone")
	}
}

func TestDeleteFile_NotFound(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "nofile.txt")

	tool := &DeleteFileTool{}
	args, _ := json.Marshal(map[string]string{"path": path})
	_, err := tool.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ── create_directory 测试 ─────────────────────────────────

func TestCreateDirectory_Success(t *testing.T) {
	dir := tempDir(t)
	subdir := filepath.Join(dir, "newdir")

	tool := &CreateDirectoryTool{}
	args, _ := json.Marshal(map[string]string{"path": subdir})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("create_directory: %v", err)
	}
	if !strings.Contains(result, "[OK]") {
		t.Errorf("expected [OK], got: %s", result)
	}

	info, err := os.Stat(subdir)
	if err != nil || !info.IsDir() {
		t.Fatal("directory should exist")
	}
}

func TestCreateDirectory_AlreadyExists(t *testing.T) {
	dir := tempDir(t)
	subdir := filepath.Join(dir, "exists")
	os.MkdirAll(subdir, 0755)

	tool := &CreateDirectoryTool{}
	args, _ := json.Marshal(map[string]string{"path": subdir})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("should succeed for existing dir: %v", err)
	}
	if !strings.Contains(result, "已存在") {
		t.Errorf("should say '已存在', got: %s", result)
	}
}

// ── list_directory 测试 ───────────────────────────────────

func TestListDirectory_Success(t *testing.T) {
	dir := tempDir(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "c.txt"), []byte("c"), 0644)

	tool := &ListDirectoryTool{}
	args, _ := json.Marshal(map[string]string{"path": dir})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("list_directory: %v", err)
	}
	if !strings.Contains(result, "a.txt") {
		t.Errorf("should contain a.txt: %s", result)
	}
	if !strings.Contains(result, "b.go") {
		t.Errorf("should contain b.go: %s", result)
	}
	if !strings.Contains(result, "sub") {
		t.Errorf("should contain sub dir: %s", result)
	}
}

func TestListDirectory_Pattern(t *testing.T) {
	dir := tempDir(t)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)

	tool := &ListDirectoryTool{}
	args, _ := json.Marshal(map[string]any{"path": dir, "pattern": "*.go"})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("list_directory: %v", err)
	}
	if !strings.Contains(result, "a.go") {
		t.Errorf("should contain a.go: %s", result)
	}
	if strings.Contains(result, "b.txt") {
		t.Errorf("should not contain b.txt with *.go pattern: %s", result)
	}
}

// ── search_files 测试 ─────────────────────────────────────

func TestSearchFiles_ByContent(t *testing.T) {
	dir := tempDir(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello golang world"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("goodbye world"), 0644)

	tool := &SearchFilesTool{}
	args, _ := json.Marshal(map[string]any{"content": "golang", "path": dir})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("search_files: %v", err)
	}
	if !strings.Contains(result, "a.txt") {
		t.Errorf("should find a.txt: %s", result)
	}
	if strings.Contains(result, "b.txt") {
		t.Errorf("b.txt should not match: %s", result)
	}
}

func TestSearchFiles_ByGlob(t *testing.T) {
	dir := tempDir(t)
	os.WriteFile(filepath.Join(dir, "test.go"), []byte("package test"), 0644)
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("text"), 0644)

	tool := &SearchFilesTool{}
	args, _ := json.Marshal(map[string]any{"pattern": "*.go", "path": dir})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("search_files: %v", err)
	}
	if !strings.Contains(result, "test.go") {
		t.Errorf("should find test.go: %s", result)
	}
}

// ── get_file_info 测试 ────────────────────────────────────

func TestGetFileInfo_Success(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "info.txt")
	os.WriteFile(path, []byte("file info test content"), 0644)

	tool := &GetFileInfoTool{}
	args, _ := json.Marshal(map[string]string{"path": path})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("get_file_info: %v", err)
	}
	if !strings.Contains(result, "info.txt") {
		t.Errorf("should contain filename: %s", result)
	}
	if !strings.Contains(result, "文件") {
		t.Errorf("should say '文件': %s", result)
	}
}

func TestGetFileInfo_Directory(t *testing.T) {
	dir := tempDir(t)

	tool := &GetFileInfoTool{}
	args, _ := json.Marshal(map[string]string{"path": dir})
	result, err := tool.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("get_file_info: %v", err)
	}
	if !strings.Contains(result, "目录") {
		t.Errorf("should say '目录': %s", result)
	}
}

// ── resolvePath 测试 ──────────────────────────────────────

func TestResolvePath_Relative(t *testing.T) {
	result, err := resolvePath("src/main.go")
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	expected := filepath.Join(defaultProjRoot, "src", "main.go")
	if result != expected {
		t.Errorf("resolved = %q, want %q", result, expected)
	}
}

func TestResolvePath_AbsoluteInProject(t *testing.T) {
	result, err := resolvePath(defaultProjRoot + `\src\main.go`)
	if err != nil {
		t.Fatalf("resolvePath: %v", err)
	}
	if result != filepath.Join(defaultProjRoot, "src", "main.go") {
		t.Errorf("resolved = %q", result)
	}
}

func TestResolvePath_BlockSystemDir(t *testing.T) {
	_, err := resolvePath(`C:\Windows\System32\cmd.exe`)
	if err == nil {
		t.Fatal("should block system directory")
	}
}

func TestResolvePath_BlockSystemDir_SegmentBoundary(t *testing.T) {
	blocked := []string{
		`C:\Windows`,           // exact segment
		`E:\proc`,              // exact segment, different drive
		`C:\Windows\Temp\x`,    // segment + children
		`C:\Program Files\app`, // spaced segment
	}
	for _, p := range blocked {
		if _, err := resolvePath(p); err == nil {
			t.Errorf("should block %s", p)
		}
	}

	allowed := []string{
		`D:\system\cleanup`, // "/sys" must not swallow "/system"
		`D:\WindowsApps\x`,  // "/windows" must not swallow "/windowsapps"
		`D:\etcdata\a`,      // "/etc" must not swallow "/etcdata"
		`D:\procmon\logs`,   // "/proc" must not swallow "/procmon"
		`D:\goagentpro\src`, // normal project path
	}
	for _, p := range allowed {
		if _, err := resolvePath(p); err != nil {
			t.Errorf("should allow %s, got: %v", p, err)
		}
	}
}

func TestResolvePath_Empty(t *testing.T) {
	_, err := resolvePath("")
	if err == nil {
		t.Fatal("empty path should error")
	}
}

// ── isSensitiveFilePath 测试 ──────────────────────────────

func TestIsSensitiveFilePath(t *testing.T) {
	if !isSensitiveFilePath(`D:\project\.env`) {
		t.Error(".env should be sensitive")
	}
	if !isSensitiveFilePath(`/path/id_rsa`) {
		t.Error("id_rsa should be sensitive")
	}
	if !isSensitiveFilePath(`/keys/secret.pem`) {
		t.Error("*.pem should be sensitive")
	}
	if isSensitiveFilePath(`D:\project\main.go`) {
		t.Error("main.go should not be sensitive")
	}
	if isSensitiveFilePath(`D:\project\readme.md`) {
		t.Error("readme.md should not be sensitive")
	}
}

// ── humanSize 测试 ────────────────────────────────────────

func TestHumanSize(t *testing.T) {
	if s := humanSize(100); s != "100 B" {
		t.Errorf("100 B: got %q", s)
	}
	if s := humanSize(1500); !strings.Contains(s, "KB") {
		t.Errorf("1500: got %q", s)
	}
	if s := humanSize(3 * 1024 * 1024); !strings.Contains(s, "MB") {
		t.Errorf("3MB: got %q", s)
	}
}
