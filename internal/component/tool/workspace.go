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
// In Docker/WSL, converts D:\foo → /mnt/d/foo (via HOST_MNT_PREFIX env).
func SetWorkspaceRoot(path string) {
	wsRootMu.Lock()
	defer wsRootMu.Unlock()
	wsRoot = hostToContainer(path)
}

// hostToContainer converts a host path to a container-readable path.
func hostToContainer(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		drive := strings.ToLower(string(p[0]))
		rest := strings.TrimLeft(p[3:], `/\`)
		if mnt := os.Getenv("HOST_MNT_PREFIX"); mnt != "" {
			return mnt + "/" + drive + "/" + rest
		}
		return "/" + strings.ToUpper(string(drive)) + "/" + rest
	}
	return p
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
	// Docker/WSL format: /mnt/d/... → D:\...
	if mnt := os.Getenv("HOST_MNT_PREFIX"); mnt != "" && strings.HasPrefix(p, mnt+"/") {
		rest := p[len(mnt)+1:]
		if len(rest) >= 2 && rest[1] == '/' {
			return strings.ToUpper(string(rest[0])) + ":\\" + strings.ReplaceAll(rest[2:], "/", "\\")
		}
	}
	// Legacy format: /D/... → D:\...
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
	// Try container path first (e.g. /mnt/d/... in Docker).
	entries, err := os.ReadDir(root)
	if err != nil {
		entries, err = os.ReadDir(toWinPath(root))
		if err != nil {
			return "(空目录)"
		}
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
	if count == 0 {
		return "(空目录)"
	}
	return b.String()
}
