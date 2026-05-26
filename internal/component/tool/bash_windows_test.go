//go:build windows

package tool

import (
	"testing"
)

func TestNewSysProcAttr_Windows(t *testing.T) {
	attr := newSysProcAttr()
	if attr == nil {
		t.Fatal("newSysProcAttr() should not return nil on Windows")
	}
	if !attr.HideWindow {
		t.Error("HideWindow should be true on Windows")
	}
}
