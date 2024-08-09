//go:build windows

package portfwd

import (
	"golang.org/x/sys/windows"
	"syscall"
)

func Control(_, _ string, c syscall.RawConn) (err error) {
	controlErr := c.Control(func(fd uintptr) {
		err = windows.SetsockoptInt(windows.Handle(int(fd)), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
		if err != nil {
			return
		}
	})
	if controlErr != nil {
		err = controlErr
	}
	return
}
