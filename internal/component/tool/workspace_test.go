package tool

import (
	"os"
	"testing"
)

func TestHostToContainer_WindowsPath(t *testing.T) {
	result := hostToContainer("D:\\project\\foo")
	// Windows 路径中的反斜杠会被保留
	expected := "/D/project\\foo"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestHostToContainer_WithMntPrefix(t *testing.T) {
	prev := os.Getenv("HOST_MNT_PREFIX")
	os.Setenv("HOST_MNT_PREFIX", "/mnt")
	defer os.Setenv("HOST_MNT_PREFIX", prev)

	result := hostToContainer("D:\\project\\foo")
	expected := "/mnt/d/project\\foo"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestHostToContainer_AlreadyUnix(t *testing.T) {
	result := hostToContainer("/home/user/project")
	if result != "/home/user/project" {
		t.Errorf("got %q, want unchanged", result)
	}
}

func TestHostToContainer_DockerFormat(t *testing.T) {
	result := hostToContainer("/D/project/foo")
	if result != "/D/project/foo" {
		t.Errorf("got %q, want unchanged", result)
	}
}

func TestToWinPath_AlreadyWindows(t *testing.T) {
	result := toWinPath("D:\\project\\foo")
	if result != "D:\\project\\foo" {
		t.Errorf("got %q, want unchanged", result)
	}
}

func TestToWinPath_LegacyFormat(t *testing.T) {
	result := toWinPath("/D/project/foo")
	expected := "D:\\project\\foo"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestToWinPath_WithMntPrefix(t *testing.T) {
	prev := os.Getenv("HOST_MNT_PREFIX")
	os.Setenv("HOST_MNT_PREFIX", "/mnt")
	defer os.Setenv("HOST_MNT_PREFIX", prev)

	result := toWinPath("/mnt/d/project/foo")
	expected := "D:\\project\\foo"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestToWinPath_NoPrefix(t *testing.T) {
	result := toWinPath("/linux/path")
	if result != "/linux/path" {
		t.Errorf("got %q, want unchanged", result)
	}
}

func TestHostToContainer_EmptyInput(t *testing.T) {
	result := hostToContainer("")
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestToWinPath_EmptyInput(t *testing.T) {
	result := toWinPath("")
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestSetAndGetWorkspaceRoot(t *testing.T) {
	wsRootMu.Lock()
	wsRoot = ""
	wsRootMu.Unlock()

	SetWorkspaceRoot("D:\\myworkspace")
	root := GetWorkspaceRoot()
	expected := "/D/myworkspace"
	if root != expected {
		t.Errorf("GetWorkspaceRoot = %q, want %q", root, expected)
	}

	win := GetWorkspaceWinPath()
	expectedWin := "D:\\myworkspace"
	if win != expectedWin {
		t.Errorf("GetWorkspaceWinPath = %q, want %q", win, expectedWin)
	}
}

func TestReadWorkspaceSummary_NoRoot(t *testing.T) {
	wsRootMu.Lock()
	old := wsRoot
	wsRoot = ""
	wsRootMu.Unlock()
	defer func() {
		wsRootMu.Lock()
		wsRoot = old
		wsRootMu.Unlock()
	}()

	summary := ReadWorkspaceSummary()
	if summary != "" {
		t.Errorf("ReadWorkspaceSummary with empty root = %q, want empty", summary)
	}
}
