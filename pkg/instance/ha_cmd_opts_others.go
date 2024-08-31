//go:build !windows

package instance

import (
	"syscall"
)

var (
	ForegroundSysProcAttr = &syscall.SysProcAttr{}
	BackgroundSysProcAttr = &syscall.SysProcAttr{Setpgid: true}
)
