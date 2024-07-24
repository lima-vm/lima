//go:build !windows

package instance

import (
	"syscall"
)

var SysProcAttr = &syscall.SysProcAttr{}
