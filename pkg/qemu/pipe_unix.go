//go:build !windows
// +build !windows

package qemu

import (
	"syscall"
)

func createFifo(path string) error {
	var stat syscall.Stat_t
	err := syscall.Stat(path, &stat)
	if err != nil {
		if err.(syscall.Errno) != syscall.ENOENT {
			return err
		}
	} else {
		if stat.Mode&syscall.S_IFMT == syscall.S_IFIFO {
			return nil
		}
	}
	return syscall.Mkfifo(path, 0600)
}

func PipeMakeFifo(path string) error {
	if err := createFifo(path + ".in"); err != nil {
		return err
	}
	if err := createFifo(path + ".out"); err != nil {
		return err
	}
	return nil
}
