//go:build !windows

package osutil

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func Ftruncate(fd int, length int64) (err error) {
	return unix.Ftruncate(fd, length)
}

func SignalName(sig os.Signal) string {
	return unix.SignalName(sig.(syscall.Signal))
}
