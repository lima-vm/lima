//go:build !windows

package osutil

import "golang.org/x/sys/unix"

func Ftruncate(fd int, length int64) (err error) {
	return unix.Ftruncate(fd, length)
}
