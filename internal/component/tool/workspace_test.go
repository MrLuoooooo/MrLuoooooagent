package tool

import (
	"os"
	"testing"
)

func TestHostToContainer_NativeWindows(t *testing.T) {
	if !isWindows {
		t.Skip("Windows-only behavior: native host keeps paths as-is")
	}
	// Native Windows host must NOT convert — conversion would corrupt every
	// file operation (files silently written to a bogus "D:\\D\\..." tree).
	if got := hostToContainer(`D:\project\foo`); got != `D:\project\foo` {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestHostToContainerUnix_DrivePath(t *testing.T) {
	got := hostToContainerUnix(`D:\project\foo`)
	if got != "/D/project/foo" {
		t.Errorf("got %q, want /D/project/foo (separators must be normalized)", got)
	}
}

func TestHostToContainerUnix_WithMntPrefix(t *testing.T) {
	prev := os.Getenv("HOST_MNT_PREFIX")
	os.Setenv("HOST_MNT_PREFIX", "/mnt")
	defer os.Setenv("HOST_MNT_PREFIX", prev)

	got := hostToContainerUnix(`D:\project\foo`)
	if got != "/mnt/d/project/foo" {
		t.Errorf("got %q, want /mnt/d/project/foo", got)
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

	SetWorkspaceRoot(`D:\myworkspace`)

	// Native Windows keeps the path as-is; Docker/Linux converts to container form.
	wantRoot := `D:\myworkspace`
	if !isWindows {
		wantRoot = "/D/myworkspace"
	}
	if root := GetWorkspaceRoot(); root != wantRoot {
		t.Errorf("GetWorkspaceRoot = %q, want %q", root, wantRoot)
	}

	// GetWorkspaceWinPath must yield the Windows form on both platforms.
	if win := GetWorkspaceWinPath(); win != `D:\myworkspace` {
		t.Errorf("GetWorkspaceWinPath = %q, want %q", win, `D:\myworkspace`)
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
