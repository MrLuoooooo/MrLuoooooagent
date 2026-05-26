//go:build !windows

package tool

import (
	"testing"
)

func TestNewSysProcAttr_Unix(t *testing.T) {
	attr := newSysProcAttr()
	if attr != nil {
		t.Error("newSysProcAttr() should return nil on non-Windows platforms")
	}
}
