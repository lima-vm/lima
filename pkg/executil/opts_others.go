//go:build !windows

package executil

import (
	"syscall"
)

var (
	ForegroundSysProcAttr = &syscall.SysProcAttr{}
	BackgroundSysProcAttr = &syscall.SysProcAttr{Setpgid: true}
)
