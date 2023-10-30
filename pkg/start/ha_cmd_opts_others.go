//go:build !windows

package start

import (
	"syscall"
)

var SysProcAttr = &syscall.SysProcAttr{}
