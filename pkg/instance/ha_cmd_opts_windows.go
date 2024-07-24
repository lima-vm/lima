package instance

import (
	"syscall"
)

var SysProcAttr = &syscall.SysProcAttr{
	CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
}
