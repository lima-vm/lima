//go:build !windows

/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
