package instance

import (
	"syscall"
)

var (
	ForegroundSysProcAttr = &syscall.SysProcAttr{}
	BackgroundSysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
)
