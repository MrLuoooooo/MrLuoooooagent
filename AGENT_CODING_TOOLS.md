# GoAgentPro — 文件系统 + Bash 工具 Agent 开发任务书

> **用途**: 将此文件完整交付给开发 Agent，实现"像 OpenClaw/Cline 那样的文件操作+Bash执行+代码编写"能力
> **前提**: Agent Graph、Tool Registry、OpenAI Model 的 tools 传输均已就位

---

## 一、项目背景

### 当前已完成的基础设施

| 组件 | 状态 | 说明 |
|------|------|------|
| Agent ReAct 循环 (`graph/agent.go`) | ✅ | START→ChatModel→Branch→Tools→loop→END |
| Tool Registry (`component/tool/registry.go`) | ✅ | 线程安全全局注册器，`Tool{Info,InvokableRun}` 接口 |
| Tool 注册 + 绑定 (`server/fx.go:278-326`) | ✅ | `tool.Register()` + `MustBindTools()` |
| OpenAI ChatModel tools 字段传输 | ✅ | `tools` 数组注入请求体，已解析 `tool_calls` delta |
| 前端 tool_call/tool_result 事件处理 | ✅ | `useChatStream.ts` + `ToolCallBadge.tsx` |
| 现有工具 | ✅ | `DateTimeTool`, `WebSearchTool`, `RAGTool` |

### 当前缺失的能力（即本次任务目标）

- **文件系统操作**：读写文件、创建/删除目录、列出目录、搜索文件、编辑文件
- **Bash 命令执行**：在本地 shell 中执行命令并获取输出
- **代码编写**：结合文件操作 + Bash 实现完整的代码编写能力

---

## 二、技术约束（必须遵守）

### 2.1 环境

| 项目 | 值 |
|------|-----|
| OS | Windows |
| Go 版本 | 1.23 |
| Eino 版本 | v0.8.13 |
| 项目路径 | `D:\goagentpro` |
| 模块名 | `github.com/yourusername/goagentpro` |
| 桌面路径 | `C:\Users\35617\Desktop` |

### 2.2 架构约束

1. **所有新工具必须放在** `internal/component/tool/` 目录下，按功能拆文件
2. **必须实现** `tool.Tool` 接口（来自 `registry.go`）：
   ```go
   type Tool interface {
       Info(ctx context.Context) (*schema.ToolInfo, error)
       InvokableRun(ctx context.Context, argumentsInJSON string, opts ...eino_tool.Option) (string, error)
   }
   ```
3. **必须注册**：在 `server/fx.go` 的 `ProvideAgentGraph()` 中调用 `tool.Register()`
4. **参数必须用 JSON Schema** 描述，通过 `schema.NewParamsOneOfByParams()` 构建
5. **`InvokableRun` 返回纯文本字符串**，LLM 直接读
6. **禁止使用 Ant Design / MUI / Redux / Zustand**（前端约束，无关但必须记住）
7. **遵循现有命名规范**：文件 `snake_case.go`，类型 `PascalCase`，包名小写单数

---

## 三、需要实现的工具

### 3.1 文件系统工具组 (`internal/component/tool/fs.go`)

所有文件操作**默认限定到项目工作区** `D:\goagentpro`，除非 LLM 明确指定绝对路径。

#### Tool 1: `read_file` — 读取文件

```go
ToolInfo{
    Name: "read_file",
    Desc: "读取指定文件的完整内容。支持文本文件，二进制文件返回大小和 base64 摘要。",
    Params: {
        "path": {Type: String, Desc: "文件路径（相对项目目录或绝对路径）", Required: true},
        "max_size": {Type: Integer, Desc: "最大读取字节数（默认 1MB，超长文件只返回头 1MB）", Required: false},
    },
}
```

**实现逻辑**：
- `os.Stat()` 检查文件存在
- 相对路径拼接 `D:\goagentpro` + path
- `os.ReadFile()` 读取（限制 max_size）
- 二进制检测：前 512 字节检查 `utf8.Valid()`，非文本则返回 `[二进制文件 ${size} 字节]`
- 文件 > 100KB 时加警告 `[文件较大(${size}B)，仅返回前 ${limit}B]`

#### Tool 2: `write_file` — 写入文件

```go
ToolInfo{
    Name: "write_file",
    Desc: "创建新文件或覆盖已有的文件。如果文件所在目录不存在则自动创建目录。",
    Params: {
        "path": {Type: String, Desc: "文件路径", Required: true},
        "content": {Type: String, Desc: "文件内容", Required: true},
    },
}
```

**实现逻辑**：
- `filepath.Dir()` → `os.MkdirAll(0755)` 确保目录存在
- 安全检查：禁止覆盖 `.exe`, `.dll`, `.so`, `.dylib` 等二进制可执行文件
- `os.WriteFile()` 写入
- 返回 `[OK] 已写入 ${path} (${len} 字节)`

#### Tool 3: `edit_file` — 编辑文件

```go
ToolInfo{
    Name: "edit_file",
    Desc: "对已存在的文件执行精确的字符串替换。用于修改代码特定段落，不会重写整个文件。",
    Params: {
        "path": {Type: String, Desc: "文件路径", Required: true},
        "old_string": {Type: String, Desc: "要被替换的精确字符串（必须唯一匹配）", Required: true},
        "new_string": {Type: String, Desc: "替换后的新字符串", Required: true},
    },
}
```

**实现逻辑**：
- `os.ReadFile` → 转为 string
- `strings.Count(_, old_string)` 检查唯一性
  - 匹配 0 次 → 返回错误 `[FAIL] 找不到匹配内容`
  - 匹配 >1 次 → 返回错误 `[FAIL] 发现 ${n} 处匹配，请提供更多上下文以确保唯一`
- `strings.Replace(_, old_string, new_string, 1)` 替换
- `os.WriteFile` 写回
- 返回修改的 diff 摘要（前后各 50 字符上下文）

#### Tool 4: `delete_file` — 删除文件

```go
ToolInfo{
    Name: "delete_file",
    Desc: "删除指定文件。操作不可逆，请确认后再执行。",
    Params: {
        "path": {Type: String, Desc: "文件路径", Required: true},
    },
}
```

**实现逻辑**：
- `os.Stat()` 确认文件存在
- `os.Remove()` 删除
- 返回 `[OK] 已删除 ${path}`

#### Tool 5: `list_directory` — 列出目录

```go
ToolInfo{
    Name: "list_directory",
    Desc: "列出指定目录的内容（文件和子目录）。默认递归 1 层，可指定深度。",
    Params: {
        "path": {Type: String, Desc: "目录路径，空字符串表示项目根目录", Required: true},
        "depth": {Type: Integer, Desc: "递归深度（0=仅当前目录，1=一层子目录，-1=全部）", Required: false},
        "pattern": {Type: String, Desc: "文件通配符过滤（如 *.go, *.ts, **/test*）", Required: false},
    },
}
```

**实现逻辑**：
- `os.ReadDir()` 或 `filepath.WalkDir()`
- 输出格式：
  ```
  ./
  ├── go.mod
  ├── main.go (2.3 KB)
  ├── src/
  │   ├── app.ts (1.1 KB)
  │   └── utils/
  ```
- depth 控制递归层级
- pattern 用 `filepath.Match` 或 `path.Match` 过滤
- **限制**: 最大返回 500 条，超出时提示 `[TRUNCATED] 仅显示前 500 条`
- 排除 `node_modules/`, `.git/`, `vendor/`, `dist/`, `.next/` 等目录

#### Tool 6: `create_directory` — 创建目录

```go
ToolInfo{
    Name: "create_directory",
    Desc: "创建新目录（含父目录）。如果目录已存在则静默成功。",
    Params: {
        "path": {Type: String, Desc: "目录路径", Required: true},
    },
}
```

#### Tool 7: `search_files` — 搜索文件

```go
ToolInfo{
    Name: "search_files",
    Desc: "在文件系统中搜索匹配模式的文件或内容。支持 glob 文件模式搜索和文本内容搜索。",
    Params: {
        "pattern": {Type: String, Desc: "glob 文件模式，如 **/*.go", Required: false},
        "content": {Type: String, Desc: "搜索文件内容中包含此文本的文件", Required: false},
        "path": {Type: String, Desc: "搜索起点目录，默认项目根", Required: false},
        "max_results": {Type: Integer, Desc: "最大返回结果数（默认 50）", Required: false},
    },
}
```

**实现逻辑**：
- `content` 非空时 → `filepath.WalkDir` + `strings.Contains` 行扫描
- `pattern` 非空时 → `filepath.Glob` 或 `filepath.WalkDir`
- 排除 `node_modules/`, `.git/`, `vendor/`, `dist/`
- 输出格式：每行 `path:line:content`
- 限制最大结果数

#### Tool 8: `get_file_info` — 获取文件信息

```go
ToolInfo{
    Name: "get_file_info",
    Desc: "获取文件或目录的元信息（大小、修改时间、权限等）。",
    Params: {
        "path": {Type: String, Desc: "文件或目录路径", Required: true},
    },
}
```

---

### 3.2 Bash 工具组 (`internal/component/tool/bash.go`)

**安全是第一优先级**。必须做沙箱限制。

#### Tool 9: `execute_command` — 执行命令

```go
ToolInfo{
    Name: "execute_command",
    Desc: "在本地 shell 中执行一条命令并返回输出。适用于运行构建命令、代码格式化、测试和各类 CLI 操作。",
    Params: {
        "command": {Type: String, Desc: "要执行的 shell 命令", Required: true},
        "work_dir": {Type: String, Desc: "工作目录（默认项目根 D:\\goagentpro）", Required: false},
        "timeout_seconds": {Type: Integer, Desc: "超时时间秒数（默认 30，最大 120）", Required: false},
    },
}
```

**实现逻辑**：

```go
// 安全过滤 — 阻止高危命令
var blockedPrefixes = []string{
    "rm -rf /", "rm -rf /c/", "rm -rf /d/",  // 根目录删除
    "format", "fdisk", "mkfs",                    // 磁盘操作
    "dd ",                                         // 低级磁盘写入
    "shutdown", "reboot", "poweroff",              // 系统操作
    "sudo",                                        // 提权
    ":(){ :|:& };:",                               // fork 炸弹
    "chmod 777", "chmod -R",                       // 递归权限修改
    "wget ", "curl ",                              // 下载（默认阻止，见 allowlist）
}

// 命令前缀匹配禁止列表
cmdLower := strings.TrimSpace(strings.ToLower(params.Command))
for _, prefix := range blockedPrefixes {
    if strings.HasPrefix(cmdLower, prefix) {
        return "", fmt.Errorf("[SECURITY] 禁止执行高危命令: %s", prefix)
    }
}

// Windows 特殊处理
var cmd *exec.Cmd
if params.Command == "" {
    return "", fmt.Errorf("command is required")
}
// Windows 下用 cmd /c
cmd = exec.Command("cmd", "/c", params.Command)
cmd.Dir = resolvedWorkDir
cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // 不弹黑窗口

// 超时控制
timeout := 30
if params.TimeoutSeconds > 0 && params.TimeoutSeconds <= 120 {
    timeout = params.TimeoutSeconds
}
ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
defer cancel()
cmd = exec.CommandContext(ctx, "cmd", "/c", params.Command)

// 捕获 stdout + stderr
var stdout, stderr bytes.Buffer
cmd.Stdout = &stdout
cmd.Stderr = &stderr

err := cmd.Run()

// 输出大小限制（最大 100KB）
output := stdout.String()
if len(output) > 102400 {
    output = output[:102400] + "\n... [TRUNCATED: 输出超过 100KB 限制]"
}
if ctx.Err() == context.DeadlineExceeded {
    return output + "\n[ERROR: 命令执行超时]", nil
}
```

**输出格式**：
```
Exit Code: 0

<标准输出内容>
---
<标准错误内容（如果有）>
```

#### Tool 10: `write_and_execute` — 写入并执行（代码能力核心）

```go
ToolInfo{
    Name: "write_and_execute",
    Desc: "写入一个文件然后立即执行它。适用于编写脚本、运行后查看效果。支持 Go/Python/JS/Bat 脚本。",
    Params: {
        "path": {Type: String, Desc: "文件路径", Required: true},
        "content": {Type: String, Desc: "文件内容", Required: true},
        "command": {Type: String, Desc: "执行命令（留空则根据文件扩展名自动选择）", Required: false},
        "timeout_seconds": {Type: Integer, Desc: "超时秒数（默认 60）", Required: false},
    },
}
```

**自动选择执行器**：
- `.go` → `go run "$path"`
- `.py` → `python "$path"`
- `.js` → `node "$path"`
- `.bat` / `.cmd` → 直接执行
- `.sh` → `bash "$path"`
- 其他 → 需要 command 参数

---

### 3.3 注册集成（修改 `server/fx.go`）

在 `ProvideAgentGraph()` 中注册新工具：

```go
// === 文件系统工具（安全模式：限定项目目录）===
tool.Register(tool.NewFileTool())       // 单例，内部按方法名分发

// === Bash 工具 ===
tool.Register(tool.NewBashTool(
    cfg.Server.AllowedDirs,     // 允许的工作目录列表
    cfg.Server.BashTimeout,     // 默认超时
))

// === 保持现有工具 ===
tool.Register(&tool.DateTimeTool{})
tool.Register(ws)
tool.Register(tool.NewRAGTool(...))
```

---

## 四、安全约束（不可违反）

### 4.1 文件系统安全

| 规则 | 说明 |
|------|------|
| **工作区限制** | 默认限定 `D:\goagentpro`，绝对路径必须包含 `D:\goagentpro` 前缀 |
| **禁止覆盖二进制** | `.exe`, `.dll`, `.so`, `.dylib`, `.sys` 等可执行文件不可写 |
| **禁止修改配置** | `.env`, `.ssh`, `id_rsa`, `.gitconfig`, `.netrc` 等敏感配置不可读/写 |
| **大小限制** | 文件读取最大 10MB，写入最大 50MB，Bash 输出最大 100KB |
| **删除保护** | `delete_file` 必须由 LLM 确认后才执行 |

### 4.2 Bash 安全

| 规则 | 说明 |
|------|------|
| **阻止高危命令** | rm -rf /, sudo, dd, fdisk, mkfs, shutdown, reboot, chmod -R, chown -R |
| **阻止网络下载** | 默认阻止 `wget` `curl` 直接下载（LLM 可通过其他工具确认后执行） |
| **超时控制** | 最大 120 秒，防止死循环和长时间阻塞 |
| **窗口隐藏** | Windows 下设置 `HideWindow: true`，不弹黑框 |
| **git 安全** | `git push --force`, `git reset --hard` 非交互模式下仅追加 commit |

### 4.3 内容安全

| 规则 | 说明 |
|------|------|
| **隐私文件** | 禁止读取 `.ssh/*`, `id_rsa*`, `*.pem`, `*.key` |
| **系统文件** | 禁止读取 `C:\Windows\*`, `C:\Program Files\*` |
| **token/密码** | 如果 `content` 中出现 `api_key`, `password`, `token`, `secret`，在返回给 LLM 时打码为 `***` |

---

## 五、实现步骤（执行顺序）

### Step 1: 审查现有代码（10 分钟）

- 读取 `internal/component/tool/registry.go` — 确认 Tool 接口
- 读取 `internal/component/tool/datetime.go` — 作为实现参考
- 读取 `internal/server/fx.go:278-326` — 确认注册位置
- 读取 `internal/component/openaimodel/openai_chat_model.go` — 确认 tools 传输
- 读取 `internal/graph/agent.go` — 确认 Agent 图结构

### Step 2: 实现 fs.go — 文件系统工具（40 分钟）

- 实现 `FileSystemTool` 结构体
- 实现 8 个文件操作方法（内部路由）
- 添加工区限制和安全检查
- 添加每个方法的测试用例

### Step 3: 实现 bash.go — Bash 工具（30 分钟）

- 实现 `BashTool` 结构体
- 实现命令执行、安全过滤、超时控制
- 实现 `write_and_execute`
- 添加测试用例

### Step 4: 注册 + 编译验证（10 分钟）

- 在 `server/fx.go` 注册新工具
- `go build ./...` 验证编译通过
- `go vet ./...` 静态检查

### Step 5: 编写测试（20 分钟）

每个工具至少包含：
- 正常路径测试（创建文件→读文件→编辑文件→删除文件）
- 错误路径测试（文件不存在、路径越界、权限不足）
- 安全拦截测试（写入 `.exe` 应拒绝、执行 `rm -rf /` 应拒绝）

---

## 六、输出要求

1. 每实现一个工具用 `✅/❌/🟡` 表格状态汇报
2. 每个工具的 `InvokableRun` 输出完整的代码
3. 所有 `Info()` 方法返回完整的 JSON Schema
4. 三次迭代：实现 → 测试 → 审查

---

## 七、参考代码模板

```go
// internal/component/tool/fs.go
package tool

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/schema"
)

// FileSystemTool provides file and directory operations.
type FileSystemTool struct {
    allowedPrefixes []string // allowed directory prefixes
}

// NewFileTool creates a new file system tool with sandbox constraints.
func NewFileTool() *FileSystemTool {
    return &FileSystemTool{
        allowedPrefixes: []string{
            "D:\\goagentpro",
            filepath.Clean("D:\\goagentpro"),
        },
    }
}

// 以下 8 个方法：Info + InvokableRun（内部路由到不同操作）
// 操作模式通过 InvokableRun 解析参数中的 "operation" 字段分发
// 或者拆成独立文件 + 在提供者层级分别注册
```

---

> **最终提示**: 这个 Agent 的 Code 能力 = 文件操作 + Bash 执行 + LLM 推理。  
> 核心是三步：
> 1. LLM 决定要做什么操作 → 调用对应工具  
> 2. 工具返回结果 → LLM 分析输出  
> 3. LLM 决定下一步 → 调用下一个工具或生成最终响应  
>
> 工具本身只做"执行"，不做"决策"。FileSystemTool 和 BashTool 是 LLM 的"手和眼睛"，LLM 是"大脑"。
