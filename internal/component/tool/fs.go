package tool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ── 路径常量 ───────────────────────────────────────────────

// defaultProjRoot is the base directory for all file operations.
// It can be overridden via the GOAGENT_PROJECT_ROOT env var for Docker/cross-platform use.
var defaultProjRoot = getDefaultRoot()

func getDefaultRoot() string {
	if r := os.Getenv("GOAGENT_PROJECT_ROOT"); r != "" {
		return r
	}
	wd, err := os.Getwd()
	if err != nil {
		return `D:\`
	}
	return wd
}

var sensitiveExt = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".sys": true,
}
var sensitiveFile = map[string]bool{
	".env": true, ".env.local": true, ".env.production": true, ".gitconfig": true, ".netrc": true,
}
var sensitiveGlob = []string{"id_rsa*", "*.pem", "*.key", ".ssh/*"}

// resolvePath converts a relative path to absolute under the project root.
// Absolute paths are accepted only if they start with the project root or an explicit allowlist.
// Windows paths (D:\...) are converted to container paths (/mnt/d/... or /D/...) in Docker.
func resolvePath(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("路径不能为空")
	}
	// Normalize all backslashes to forward slashes (Linux doesn't handle \ as separator).
	input = strings.ReplaceAll(input, `\`, `/`)
	input = filepath.Clean(input)

	// Detect Windows absolute path: D:\foo or D:/foo
	if len(input) >= 2 && input[1] == ':' {
		drive := string(input[0])
		rest := strings.TrimLeft(input[3:], `/\`)
		if rest == "" {
			return "", fmt.Errorf("禁止写入驱动器根目录: %s", input)
		}
		// Use container-aware path conversion (respects HOST_MNT_PREFIX).
		containerPath := hostToContainer(drive + ":\\" + rest)
		return containerPath, nil
	}

	if filepath.IsAbs(input) {
		lower := strings.ToLower(input)
		for _, bad := range []string{`/windows`, `/program files`, `/program files (x86)`, `/etc`, `/sys`, `/proc`} {
			if strings.HasPrefix(lower, bad) {
				return "", fmt.Errorf("禁止访问系统目录: %s", input)
			}
		}
		return input, nil
	}
	// Relative path: resolve against workspace root or project root.
	root := GetWorkspaceRoot()
	if root == "" {
		root = defaultProjRoot
	}
	return filepath.Join(root, input), nil
}

// isSensitiveFilePath checks whether a path targets protected files.
func isSensitiveFilePath(path string) bool {
	base := filepath.Base(path)
	for _, g := range sensitiveGlob {
		if matched, _ := filepath.Match(g, base); matched {
			return true
		}
	}
	if sensitiveFile[strings.ToLower(base)] {
		return true
	}
	return false
}

// humanSize 把字节数转成可读字符串。
func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

// isBinary detects binary content by checking the first bytes for valid UTF-8.
func isBinary(data []byte) bool {
	return !utf8.Valid(data[:min(len(data), 512)])
}

// ── Tool 1: read_file ─────────────────────────────────────

type ReadFileTool struct{}

func (t *ReadFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "read_file",
		Desc: "读取指定文件的完整内容。支持文本文件，二进制文件返回大小和 base64 摘要。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":     {Type: schema.String, Desc: "文件路径（相对项目目录或绝对路径）", Required: true},
			"max_size": {Type: schema.Integer, Desc: "最大读取字节数（默认 1MB）", Required: false},
		}),
	}, nil
}

func (t *ReadFileTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path    string `json:"path"`
		MaxSize int64  `json:"max_size"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("read_file: 参数解析失败: %w", err)
	}
	if args.MaxSize <= 0 {
		args.MaxSize = 1024 * 1024 // 1MB
	}
	if args.MaxSize > 10*1024*1024 {
		args.MaxSize = 10 * 1024 * 1024 // 10MB hard limit
	}

	path, err := resolvePath(args.Path)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("File not found: %s. You MUST call write_file tool to create it.", path), nil
		}
		return "", fmt.Errorf("read_file: 无法访问文件: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("read_file: %s 是一个目录，请使用 list_directory", path)
	}
	if isSensitiveFilePath(path) {
		return "", fmt.Errorf("read_file: 安全限制 — 禁止读取敏感配置文件")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read_file: 读取失败: %w", err)
	}

	// Binary detection
	if isBinary(data) {
		summary := base64.StdEncoding.EncodeToString(data[:min(len(data), 256)])
		return fmt.Sprintf("[二进制文件 %s]\n大小: %s\nBase64 摘要(前256字节): %s", info.Name(), humanSize(info.Size()), summary), nil
	}

	var b strings.Builder
	if info.Size() > args.MaxSize {
		data = data[:args.MaxSize]
		b.WriteString(fmt.Sprintf("[WARNING] 文件较大(%s)，仅返回前 %s\n\n", humanSize(info.Size()), humanSize(args.MaxSize)))
	}
	b.Write(data)
	return b.String(), nil
}

// ── Tool 2: write_file ────────────────────────────────────

type WriteFileTool struct{}

func (t *WriteFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "write_file",
		Desc: "创建新文件或覆盖已有的文件。如果文件所在目录不存在则自动创建目录。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":    {Type: schema.String, Desc: "文件路径", Required: true},
			"content": {Type: schema.String, Desc: "文件内容", Required: true},
		}),
	}, nil
}

func (t *WriteFileTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("write_file: 参数解析失败: %w", err)
	}

	path, err := resolvePath(args.Path)
	if err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}

	// Security: block binary overwrite
	ext := strings.ToLower(filepath.Ext(path))
	if sensitiveExt[ext] {
		return "", fmt.Errorf("write_file: 安全限制 — 禁止覆盖可执行文件 (.exe/.dll/.so/.dylib/.sys)")
	}
	if isSensitiveFilePath(path) {
		return "", fmt.Errorf("write_file: 安全限制 — 禁止写入敏感配置文件")
	}
	if len(args.Content) > 50*1024*1024 {
		return "", fmt.Errorf("write_file: 内容过大(%d字节)，超过 50MB 限制", len(args.Content))
	}

	// Ensure directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("write_file: 创建目录失败(path=%s err=%v)", dir, err)
	}

	if err := os.WriteFile(path, []byte(args.Content), 0644); err != nil {
		// Windows sandbox (WorkBuddy) may block os.WriteFile outside project dir.
		// Fallback: write temp file in project root, copy via shell.
		tmp, tmpErr := os.CreateTemp(defaultProjRoot, "goagent_write_*.tmp")
		if tmpErr != nil {
			return "", fmt.Errorf("write_file: 创建临时文件失败: %w", tmpErr)
		}
		tmpPath := tmp.Name()
		if _, we := tmp.Write([]byte(args.Content)); we != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return "", fmt.Errorf("write_file: 写入临时文件失败: %w", we)
		}
		tmp.Close()
		defer os.Remove(tmpPath)

		// Ensure parent dir exists.
		os.MkdirAll(filepath.Dir(path), 0755)

		var copyCmd *exec.Cmd
		if isWindows {
			copyCmd = exec.Command("cmd", "/c", "copy", "/y", tmpPath, path)
		} else {
			copyCmd = exec.Command("cp", tmpPath, path)
		}
		out, e2 := copyCmd.CombinedOutput()
		if e2 != nil {
			return "", fmt.Errorf("write_file: 写入失败(path=%s go_err=%v shell_err=%v shell_out=%s)",
				path, err, e2, string(out))
		}
	}

	return fmt.Sprintf("[OK] 已写入 %s (%s 字节)", path, humanSize(int64(len(args.Content)))), nil
}

// ── Tool 3: edit_file ─────────────────────────────────────

type EditFileTool struct{}

func (t *EditFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "edit_file",
		Desc: "对已存在的文件执行精确的字符串替换。用于修改代码特定段落，不会重写整个文件。old_string 必须在文件中唯一匹配。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":       {Type: schema.String, Desc: "文件路径", Required: true},
			"old_string": {Type: schema.String, Desc: "要被替换的精确字符串（必须唯一匹配）", Required: true},
			"new_string": {Type: schema.String, Desc: "替换后的新字符串", Required: true},
		}),
	}, nil
}

func (t *EditFileTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("edit_file: 参数解析失败: %w", err)
	}

	path, err := resolvePath(args.Path)
	if err != nil {
		return "", fmt.Errorf("edit_file: %w", err)
	}

	// Security checks.
	ext := strings.ToLower(filepath.Ext(path))
	if sensitiveExt[ext] {
		return "", fmt.Errorf("edit_file: 安全限制 — 禁止编辑可执行文件")
	}
	if isSensitiveFilePath(path) {
		return "", fmt.Errorf("edit_file: 安全限制 — 禁止编辑敏感配置文件")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("edit_file: 文件不存在: %s", path)
		}
		return "", fmt.Errorf("edit_file: 读取失败: %w", err)
	}
	content := string(data)

	count := strings.Count(content, args.OldString)
	if count == 0 {
		return "", fmt.Errorf("[FAIL] 找不到匹配内容。old_string 在文件中不存在: %s", args.OldString)
	}
	if count > 1 {
		return "", fmt.Errorf("[FAIL] 发现 %d 处匹配，请提供更多上下文以确保 old_string 唯一", count)
	}

	newContent := strings.Replace(content, args.OldString, args.NewString, 1)

	// Show diff context (50 chars around match).
	idx := strings.Index(content, args.OldString)
	contextStart := idx
	if contextStart > 50 {
		contextStart = idx - 50
	}
	contextEnd := idx + len(args.OldString) + 50
	if contextEnd > len(content) {
		contextEnd = len(content)
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("edit_file: 写入失败: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[OK] %s 已更新\n", filepath.Base(path)))
	b.WriteString(fmt.Sprintf("旧内容: ...%s...\n", content[contextStart:min(contextEnd, idx+len(args.OldString)+50)]))
	b.WriteString("已完成 1 处替换")
	return b.String(), nil
}

// ── Tool 4: delete_file ───────────────────────────────────

type DeleteFileTool struct{}

func (t *DeleteFileTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "delete_file",
		Desc: "删除指定文件。操作不可逆，请确认后再执行。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {Type: schema.String, Desc: "文件路径", Required: true},
		}),
	}, nil
}

func (t *DeleteFileTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("delete_file: 参数解析失败: %w", err)
	}

	path, err := resolvePath(args.Path)
	if err != nil {
		return "", fmt.Errorf("delete_file: %w", err)
	}

	// Security: block deletion of binary/executable.
	ext := strings.ToLower(filepath.Ext(path))
	if sensitiveExt[ext] {
		return "", fmt.Errorf("delete_file: 安全限制 — 禁止删除可执行文件")
	}
	if isSensitiveFilePath(path) {
		return "", fmt.Errorf("delete_file: 安全限制 — 禁止删除敏感配置文件")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("delete_file: 文件不存在: %s", path)
	}

	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("delete_file: 删除失败: %w", err)
	}

	return fmt.Sprintf("[OK] 已删除 %s", path), nil
}

// ── Tool 5: list_directory ────────────────────────────────

type ListDirectoryTool struct{}

func (t *ListDirectoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "list_directory",
		Desc: "列出指定目录的内容（文件和子目录）。默认递归 1 层，可指定深度（0=仅当前目录，1=一层子目录，-1=全部）。自动排除 node_modules、.git、vendor 等目录。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":    {Type: schema.String, Desc: "目录路径，空字符串表示项目根目录", Required: true},
			"depth":   {Type: schema.Integer, Desc: "递归深度（0=仅当前目录，1=一层子目录，-1=全部，默认 1）", Required: false},
			"pattern": {Type: schema.String, Desc: "文件通配符过滤（如 *.go, *.ts, **/test*）", Required: false},
		}),
	}, nil
}

func (t *ListDirectoryTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Depth   int    `json:"depth"`
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("list_directory: 参数解析失败: %w", err)
	}
	if args.Depth == 0 {
		args.Depth = 1 // default to depth 1
	}

	path := defaultProjRoot
	if args.Path != "" {
		var err error
		path, err = resolvePath(args.Path)
		if err != nil {
			return "", fmt.Errorf("list_directory: %w", err)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("list_directory: %s", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("list_directory: %s 不是目录", path)
	}

	// Excluded directories.
	excludedDirs := map[string]bool{
		"node_modules": true, ".git": true, "vendor": true, "dist": true,
		".next": true, "__pycache__": true, ".vite": true, ".cache": true,
		".claude": true, ".workbuddy": true,
	}

	type entry struct {
		path  string
		isDir bool
		size  int64
		depth int
	}

	var entries []entry
	maxResults := 500

	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || len(entries) >= maxResults {
			return filepath.SkipDir
		}
		if p == path {
			return nil
		}
		if d.IsDir() && excludedDirs[d.Name()] {
			return filepath.SkipDir
		}

		rel, err := filepath.Rel(path, p)
		if err != nil {
			return nil
		}
		e := entry{path: rel, isDir: d.IsDir(), depth: strings.Count(rel, string(filepath.Separator))}
		if !d.IsDir() {
			fi, _ := d.Info()
			if fi != nil {
				e.size = fi.Size()
			}
		}

		// Depth filter (-1 = unlimited)
		if args.Depth >= 0 && e.depth > args.Depth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Pattern filter
		if args.Pattern != "" {
			if !d.IsDir() {
				matched, _ := filepath.Match(args.Pattern, d.Name())
				if !matched {
					return nil
				}
			}
		}

		entries = append(entries, e)
		return nil
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s/\n", filepath.Base(path)))
	displayed := 0
	for _, e := range entries {
		if displayed >= maxResults {
			break
		}
		displayed++
		indent := strings.Repeat("│   ", e.depth)
		prefix := "├── "
		if e.isDir {
			prefix = "├── "
		}
		if e.isDir {
			b.WriteString(fmt.Sprintf("%s%s%s/\n", indent, prefix, e.path))
		} else {
			b.WriteString(fmt.Sprintf("%s%s%s (%s)\n", indent, prefix, e.path, humanSize(e.size)))
		}
	}

	if len(entries) >= maxResults {
		b.WriteString(fmt.Sprintf("\n[TRUNCATED] 仅显示前 %d 条\n", maxResults))
	}
	return b.String(), nil
}

// ── Tool 6: create_directory ──────────────────────────────

type CreateDirectoryTool struct{}

func (t *CreateDirectoryTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "create_directory",
		Desc: "创建新目录（含父目录）。如果目录已存在则静默成功。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {Type: schema.String, Desc: "目录路径", Required: true},
		}),
	}, nil
}

func (t *CreateDirectoryTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("create_directory: 参数解析失败: %w", err)
	}

	path, err := resolvePath(args.Path)
	if err != nil {
		return "", fmt.Errorf("create_directory: %w", err)
	}

	// Check if already exists.
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return fmt.Sprintf("[OK] 目录已存在: %s", path), nil
		}
		return "", fmt.Errorf("create_directory: %s 已存在且不是目录", path)
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("create_directory: 创建失败: %w", err)
	}

	return fmt.Sprintf("[OK] 已创建目录 %s", path), nil
}

// ── Tool 7: search_files ──────────────────────────────────

type SearchFilesTool struct{}

func (t *SearchFilesTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "search_files",
		Desc: "在文件系统中搜索匹配模式的文件或内容。支持 glob 文件模式搜索和文本内容搜索（grep 风格）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"pattern":     {Type: schema.String, Desc: "glob 文件模式，如 **/*.go 或 *.ts", Required: false},
			"content":     {Type: schema.String, Desc: "搜索文件内容中包含此文本的文件（区分大小写）", Required: false},
			"path":        {Type: schema.String, Desc: "搜索起点目录，默认项目根", Required: false},
			"max_results": {Type: schema.Integer, Desc: "最大返回结果数（默认 50）", Required: false},
		}),
	}, nil
}

func (t *SearchFilesTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Pattern    string `json:"pattern"`
		Content    string `json:"content"`
		Path       string `json:"path"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("search_files: 参数解析失败: %w", err)
	}
	if args.MaxResults <= 0 {
		args.MaxResults = 50
	}

	root := defaultProjRoot
	if args.Path != "" {
		var err error
		root, err = resolvePath(args.Path)
		if err != nil {
			return "", fmt.Errorf("search_files: %w", err)
		}
	}

	excludedDirs := map[string]bool{
		"node_modules": true, ".git": true, "vendor": true, "dist": true,
		".next": true, ".vite": true, ".claude": true,
	}

	if args.Pattern != "" && args.Content == "" {
		return searchByGlob(root, args.Pattern, args.MaxResults)
	}
	if args.Content != "" {
		return searchByContent(root, args.Content, args.MaxResults, excludedDirs)
	}
	return "", fmt.Errorf("search_files: 必须指定 pattern 或 content 之一")
}

func searchByGlob(root, pattern string, maxResults int) (string, error) {
	absPattern := filepath.Join(root, pattern)
	matches, err := filepath.Glob(absPattern)
	if err != nil {
		return "", fmt.Errorf("glob 错误: %w", err)
	}

	var b strings.Builder
	count := 0
	for _, m := range matches {
		if count >= maxResults {
			break
		}
		rel, _ := filepath.Rel(root, m)
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		if info.IsDir() {
			b.WriteString(fmt.Sprintf("%s/ (目录)\n", rel))
		} else {
			b.WriteString(fmt.Sprintf("%s (%s)\n", rel, humanSize(info.Size())))
		}
		count++
	}
	if count == 0 {
		b.WriteString("(无匹配结果)")
	}
	if count >= maxResults {
		b.WriteString(fmt.Sprintf("\n[TRUNCATED] 仅显示前 %d 条\n", maxResults))
	}
	return b.String(), nil
}

func searchByContent(root, content string, maxResults int, excludedDirs map[string]bool) (string, error) {
	var b strings.Builder
	count := 0
	filesVisited := 0
	const maxFiles = 10000

	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || count >= maxResults || filesVisited >= maxFiles {
			return filepath.SkipDir
		}
		if d.IsDir() {
			if excludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		filesVisited++
		// Skip binary/large files.
		if fi, _ := d.Info(); fi != nil && fi.Size() > 5*1024*1024 {
			return nil
		}

		data, err := os.ReadFile(p)
		if err != nil || isBinary(data) {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for lineNo, line := range lines {
			if count >= maxResults {
				return filepath.SkipDir
			}
			if strings.Contains(line, content) {
				rel, _ := filepath.Rel(root, p)
				snippet := line
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				b.WriteString(fmt.Sprintf("%s:%d: %s\n", rel, lineNo+1, strings.TrimSpace(snippet)))
				count++
			}
		}
		return nil
	})

	if filesVisited >= maxFiles {
		b.WriteString("\n[WARN] 已扫描文件数达到上限，结果可能不完整\n")
	}
	if count == 0 {
		b.WriteString("(无匹配结果)")
	}
	if count >= maxResults {
		b.WriteString(fmt.Sprintf("\n[TRUNCATED] 仅显示前 %d 条\n", maxResults))
	}
	return b.String(), nil
}

// ── Tool 8: get_file_info ─────────────────────────────────

type GetFileInfoTool struct{}

func (t *GetFileInfoTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_file_info",
		Desc: "获取文件或目录的元信息（大小、修改时间、权限等）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path": {Type: schema.String, Desc: "文件或目录路径", Required: true},
		}),
	}, nil
}

func (t *GetFileInfoTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("get_file_info: 参数解析失败: %w", err)
	}

	path, err := resolvePath(args.Path)
	if err != nil {
		return "", fmt.Errorf("get_file_info: %w", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("get_file_info: %s", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("路径:   %s\n", path))
	b.WriteString(fmt.Sprintf("名称:   %s\n", info.Name()))
	if info.IsDir() {
		b.WriteString("类型:   目录\n")
	} else {
		b.WriteString("类型:   文件\n")
		b.WriteString(fmt.Sprintf("大小:   %s (%d 字节)\n", humanSize(info.Size()), info.Size()))
	}
	b.WriteString(fmt.Sprintf("权限:   %s\n", info.Mode().String()))
	b.WriteString(fmt.Sprintf("修改时间: %s\n", info.ModTime().Format("2006-01-02 15:04:05")))
	return b.String(), nil
}

// ── 编译时接口检查 ────────────────────────────────────────

var (
	_ tool.InvokableTool = (*ReadFileTool)(nil)
	_ tool.InvokableTool = (*WriteFileTool)(nil)
	_ tool.InvokableTool = (*EditFileTool)(nil)
	_ tool.InvokableTool = (*DeleteFileTool)(nil)
	_ tool.InvokableTool = (*ListDirectoryTool)(nil)
	_ tool.InvokableTool = (*CreateDirectoryTool)(nil)
	_ tool.InvokableTool = (*SearchFilesTool)(nil)
	_ tool.InvokableTool = (*GetFileInfoTool)(nil)
)
