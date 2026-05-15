package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ── 安全过滤规则（基于 tokenize 后的命令检查，防止空格/引号绕过）──

var blockedCommands = map[string]string{
	"rm":       "禁止使用 rm 删除文件",
	"rmdir":    "禁止使用 rmdir 删除目录",
	"del":      "禁止使用 del 删除文件",
	"rd":       "禁止使用 rd 删除目录",
	"format":   "禁止磁盘格式化操作",
	"fdisk":    "禁止磁盘分区操作",
	"mkfs":     "禁止创建文件系统",
	"dd":       "禁止低级磁盘写入",
	"shutdown": "禁止系统关机",
	"reboot":   "禁止系统重启",
	"poweroff": "禁止系统关机",
	"halt":     "禁止系统停止",
	"sudo":     "禁止权限提升",
	"chmod":    "禁止修改文件权限",
	"chown":    "禁止修改文件所有者",
	"curl":     "禁止网络下载（仅允许 localhost）",
	"wget":     "禁止网络下载（仅允许 localhost）",
	"reg":      "禁止修改注册表",
}

// isSafeCommand tokenizes the raw command string and checks ALL tokens
// against the blocklist (prevents shell chaining bypass via && | ; etc.).
// Returns (true, "") if allowed, (false, reason) if blocked.
func isSafeCommand(command string) (bool, string) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false, "空命令"
	}

	// Tokenize: split on whitespace, strip surrounding quotes from each token.
	rawTokens := strings.Fields(trimmed)
	tokens := make([]string, 0, len(rawTokens))
	for _, t := range rawTokens {
		t = strings.Trim(t, `"'`)
		if t != "" {
			tokens = append(tokens, strings.ToLower(t))
		}
	}
	if len(tokens) == 0 {
		return false, "空命令"
	}

	// Detect shell metacharacters that allow command chaining
	// and check that no command following a metachar is blocked.
	for _, t := range tokens {
		if t == "&" || t == "&&" || t == "|" || t == "||" || t == ";" {
			return false, "禁止使用 shell 链接符 (& | || && ;)，请逐条执行命令"
		}
	}

	// Skip leading cmd /c or cmd /k wrappers.
	startIdx := 0
	if tokens[0] == "cmd" && len(tokens) > startIdx+2 {
		next := tokens[1]
		if next == "/c" || next == "/k" {
			startIdx = 2
		}
	}

	// Check EVERY token against the blocklist (not just the first command).
	for i := startIdx; i < len(tokens); i++ {
		token := tokens[i]
		skipCheck := false
		if strings.HasPrefix(token, "-") || strings.HasPrefix(token, "/") {
			skipCheck = true // flag, not a command
		}
		if !skipCheck {
			if reason, blocked := blockedCommands[token]; blocked {
				if (token == "curl" || token == "wget") && i+1 < len(tokens) {
					for _, nt := range tokens[i+1:] {
						if strings.Contains(nt, "localhost") || strings.Contains(nt, "127.0.0.1") {
							return true, ""
						}
					}
				}
				return false, reason
			}
			// Check path-qualified version.
			base := token
			if idx := strings.LastIndex(base, "/"); idx >= 0 {
				base = base[idx+1:]
			}
			if idx := strings.LastIndex(base, `\`); idx >= 0 {
				base = base[idx+1:]
			}
			base = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(base, ".exe"), ".bat"), ".cmd")
			if base != token {
				if reason, blocked := blockedCommands[base]; blocked {
					return false, reason
				}
			}
		}
	}

	// Additional checks on the full command.
	full := strings.Join(tokens[startIdx:], " ")

	// rm with recursive flag.
	if strings.Contains(full, "rm ") && (strings.Contains(full, "-rf") || strings.Contains(full, "-r -f") || strings.Contains(full, "-fr")) {
		return false, "禁止递归强制删除 (rm -rf)"
	}
	// del /f /s on root drives.
	if (strings.HasPrefix(full, "del ") || strings.HasPrefix(full, "erase ")) &&
		strings.Contains(full, "/s") &&
		(strings.Contains(full, "c:") || strings.Contains(full, "d:")) {
		return false, "禁止递归删除系统盘文件 (del /s /f)"
	}
	// Write to disk devices.
	if strings.Contains(full, "> /dev/sd") || strings.Contains(full, `>\\.\`) {
		return false, "禁止直接写入磁盘设备"
	}
	// Fork bomb pattern.
	if strings.Contains(full, ":(){ :|:& };:") {
		return false, "禁止 fork 炸弹"
	}

	return true, ""
}

// ── Tool 9: execute_command ───────────────────────────────

// BashTool executes shell commands in a sandboxed environment.
type BashTool struct {
	allowedDirs []string
	defaultDir  string
}

// NewBashTool creates a new BashTool.
func NewBashTool(allowedDirs []string) *BashTool {
	if len(allowedDirs) == 0 {
		allowedDirs = []string{defaultProjRoot}
	}
	return &BashTool{
		allowedDirs: allowedDirs,
		defaultDir:  defaultProjRoot,
	}
}

func (t *BashTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "execute_command",
		Desc: "在本地 shell 中执行一条命令并返回输出。适用于运行构建命令、代码格式化、测试和各类 CLI 操作。工作目录默认为项目根 D:\\goagentpro。输出上限 100KB。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"command":         {Type: schema.String, Desc: "要执行的 shell 命令", Required: true},
			"work_dir":        {Type: schema.String, Desc: "工作目录（默认 D:\\goagentpro）", Required: false},
			"timeout_seconds": {Type: schema.Integer, Desc: "超时秒数（默认 30 秒，最大 120 秒）", Required: false},
		}),
	}, nil
}

func (t *BashTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Command        string `json:"command"`
		WorkDir        string `json:"work_dir"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("execute_command: 参数解析失败: %w", err)
	}

	if args.Command == "" {
		return "", fmt.Errorf("execute_command: command 不能为空")
	}

	// Security: tokenize + check base command (prevents space/quote bypass).
	if safe, reason := isSafeCommand(args.Command); !safe {
		return "", fmt.Errorf("[SECURITY] %s", reason)
	}

	// Resolve work directory.
	workDir := t.defaultDir
	if args.WorkDir != "" {
		var err error
		workDir, err = resolvePath(args.WorkDir)
		if err != nil {
			return "", fmt.Errorf("execute_command: %w", err)
		}
		// Verify workDir is under an allowed directory.
		allowed := false
		for _, ad := range t.allowedDirs {
			if strings.HasPrefix(strings.ToLower(workDir), strings.ToLower(ad)) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("[SECURITY] 工作目录 %s 不在允许范围内", workDir)
		}
	}

	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		return "", fmt.Errorf("execute_command: 工作目录不存在: %s", workDir)
	}

	// Timeout control.
	timeout := 30
	if args.TimeoutSeconds > 0 && args.TimeoutSeconds <= 120 {
		timeout = args.TimeoutSeconds
	} else if args.TimeoutSeconds > 120 {
		timeout = 120
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Build command.
	var stdout, stderr bytes.Buffer
	cmd := makeCmd(ctx, args.Command, workDir, &stdout, &stderr)

	err := cmd.Run()

	// Limit output size (100KB).
	output := stdout.String()
	if len(output) > 102400 {
		output = output[:102400] + "\n... [TRUNCATED: 输出超过 100KB 限制]"
	}
	errOutput := stderr.String()
	if len(errOutput) > 102400 {
		errOutput = errOutput[:102400] + "\n... [TRUNCATED: stderr 输出超过 100KB 限制]"
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(execExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return fmt.Sprintf("Exit Code: -1\n\n%s\n\n[ERROR: 命令执行超时 (%d 秒)]", output, timeout), nil
		} else {
			exitCode = -1
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Exit Code: %d\n", exitCode))
	if output != "" {
		b.WriteString("\n")
		b.WriteString(output)
	}
	if errOutput != "" {
		b.WriteString("\n---\n")
		b.WriteString(errOutput)
	}

	return b.String(), nil
}

// ── Tool 10: write_and_execute ────────────────────────────

// WriteAndExecuteTool writes a file and immediately executes it.
type WriteAndExecuteTool struct {
	bash *BashTool
}

// NewWriteAndExecuteTool creates a WriteAndExecuteTool.
func NewWriteAndExecuteTool(allowedDirs []string) *WriteAndExecuteTool {
	return &WriteAndExecuteTool{
		bash: NewBashTool(allowedDirs),
	}
}

var extToRunner = map[string]string{
	".go":  "go run %s",
	".py":  "python %s",
	".js":  "node %s",
	".bat": "%s",
	".cmd": "%s",
	".sh":  "bash %s",
}

func (t *WriteAndExecuteTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "write_and_execute",
		Desc: "写入一个文件然后立即执行它。适用于编写脚本、运行后查看效果。支持 Go/Python/JS/Bat/Bash 脚本。command 留空则根据文件扩展名自动选择执行器。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"path":            {Type: schema.String, Desc: "文件路径", Required: true},
			"content":         {Type: schema.String, Desc: "文件内容", Required: true},
			"command":         {Type: schema.String, Desc: "执行命令（留空则根据文件扩展名自动选择）", Required: false},
			"timeout_seconds": {Type: schema.Integer, Desc: "超时秒数（默认 60，最大 120）", Required: false},
		}),
	}, nil
}

func (t *WriteAndExecuteTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Path           string `json:"path"`
		Content        string `json:"content"`
		Command        string `json:"command"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("write_and_execute: 参数解析失败: %w", err)
	}

	// Step 1: Write file.
	writeTool := &WriteFileTool{}
	writeJSON, err := json.Marshal(map[string]any{
		"path":    args.Path,
		"content": args.Content,
	})
	if err != nil {
		return "", fmt.Errorf("write_and_execute: marshal write args: %w", err)
	}
	writeResult, err := writeTool.InvokableRun(ctx, string(writeJSON), opts...)
	if err != nil {
		return "", fmt.Errorf("write_and_execute: 写入失败: %w", err)
	}

	// Step 2: Resolve path.
	path, err := resolvePath(args.Path)
	if err != nil {
		return "", fmt.Errorf("write_and_execute: %w", err)
	}

	// Step 3: Determine command.
	cmd := args.Command
	if cmd == "" {
		ext := strings.ToLower(filepath.Ext(path))
		runner, ok := extToRunner[ext]
		if !ok {
			return "", fmt.Errorf("write_and_execute: 无法识别文件类型 %s，请通过 command 参数指定执行命令", ext)
		}
		cmd = fmt.Sprintf(runner, path)
	}

	if args.TimeoutSeconds <= 0 {
		args.TimeoutSeconds = 60
	}

	// Step 4: Execute.
	execJSON, err := json.Marshal(map[string]any{
		"command":         cmd,
		"timeout_seconds": args.TimeoutSeconds,
	})
	if err != nil {
		return "", fmt.Errorf("write_and_execute: marshal exec args: %w", err)
	}
	execResult, err := t.bash.InvokableRun(ctx, string(execJSON), opts...)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("[WRITE] %s\n", writeResult))
	if err != nil {
		b.WriteString(fmt.Sprintf("[RUN ERROR] %v\n", err))
	} else {
		b.WriteString(fmt.Sprintf("[RUN]\n%s", execResult))
	}
	return b.String(), nil
}

// ── 辅助函数 ──────────────────────────────────────────────

// makeCmd creates an exec.Cmd with the appropriate shell for the current OS.
func makeCmd(ctx context.Context, command, workDir string, stdout, stderr *bytes.Buffer) *exec.Cmd {
	var c *exec.Cmd
	if isWindows {
		c = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", command)
	}
	c.SysProcAttr = newSysProcAttr()
	c.Dir = workDir
	c.Stdout = stdout
	c.Stderr = stderr
	return c
}

// isWindows reports whether the current runtime is Windows.
var isWindows = checkWindows()

func checkWindows() bool {
	return len(`\`) == 1 // backslash is a valid OS path separator only on Windows
}

// execExitError is satisfied by *exec.ExitError for extracting exit codes.
type execExitError interface {
	ExitCode() int
}

var _ execExitError = (*exec.ExitError)(nil)

// ── 编译时接口检查 ────────────────────────────────────────

var (
	_ tool.InvokableTool = (*BashTool)(nil)
	_ tool.InvokableTool = (*WriteAndExecuteTool)(nil)
)
