//go:build !windows

package tool

import "syscall"

func newSysProcAttr() *syscall.SysProcAttr {
	return nil
}
