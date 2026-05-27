package tool

import (
	"os"
	"strings"
	"sync"
)

var (
	wsRoot    string
	wsRootMu  sync.RWMutex
)

// SetWorkspaceRoot sets the agent's working directory root.
// Accepts Windows paths (D:\foo) or container paths (/D/foo).
func SetWorkspaceRoot(path string) {
	wsRootMu.Lock()
	defer wsRootMu.Unlock()
	// Convert Windows path to container path.
	if len(path) >= 2 && path[1] == ':' {
		path = "/" + strings.ToUpper(string(path[0])) + "/" + strings.TrimLeft(path[3:], `/\`)
	}
	wsRoot = path
}

// GetWorkspaceRoot returns the raw workspace root path.
func GetWorkspaceRoot() string {
	wsRootMu.RLock()
	defer wsRootMu.RUnlock()
	return wsRoot
}

// GetWorkspaceWinPath returns the workspace root in Windows format.
// On Windows it converts forward-slash paths like /D/... to D:\...
func GetWorkspaceWinPath() string {
	wsRootMu.RLock()
	root := wsRoot
	wsRootMu.RUnlock()
	if root == "" {
		return ""
	}
	return toWinPath(root)
}

func toWinPath(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return p
	}
	// Handle /D/... or \D\... format (container or Windows-style with drive letter)
	if len(p) >= 3 && (p[0] == '/' || p[0] == '\\') && (p[2] == '/' || p[2] == '\\') {
		drive := p[1]
		if (drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z') {
			rest := strings.TrimLeft(p[3:], "/\\")
			return strings.ToUpper(string(drive)) + ":\\" + strings.ReplaceAll(rest, "/", "\\")
		}
	}
	return p
}

// ReadWorkspaceSummary 读工作目录摘要，注入 system prompt。
// 刻意简洁，避免分散模型对工具调用的注意力。
func ReadWorkspaceSummary() string {
	wsRootMu.RLock()
	root := wsRoot
	wsRootMu.RUnlock()
	if root == "" {
		return ""
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	var b strings.Builder
	count := 0
	for _, e := range entries {
		if count >= 15 {
			b.WriteString("...\n")
			break
		}
		if e.IsDir() {
			b.WriteString("  📁 " + e.Name() + "/\n")
		} else {
			b.WriteString("  📄 " + e.Name() + "\n")
		}
		count++
	}
	return b.String()
}
