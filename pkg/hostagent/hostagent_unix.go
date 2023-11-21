//go:build !windows

package hostagent

import (
	"syscall"

	"github.com/sirupsen/logrus"
)

// Default nofile limit is too low on some system.
// For example in the macOS standard terminal is 256.
// It means that there are only ~240 connections available from the host to the vm.
// That function increases the nofile limit for child processes, especially the ssh process
//
// More about limits in go: https://go.dev/issue/46279
func adjustNofileRlimit() {
	var limit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit); err != nil {
		logrus.WithError(err).Debug("failed to get RLIMIT_NOFILE")
	} else if limit.Cur != limit.Max {
		limit.Cur = limit.Max
		err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &limit)
		if err != nil {
			logrus.WithError(err).Debugf("failed to set RLIMIT_NOFILE (%+v)", limit)
		}
	}
}
