//go:build !windows

package osutil

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func Dup2(oldfd, newfd int) (err error) {
	return unix.Dup2(oldfd, newfd)
}

func SignalName(sig os.Signal) string {
	return unix.SignalName(sig.(syscall.Signal))
}
