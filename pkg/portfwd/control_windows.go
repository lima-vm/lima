//go:build windows

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
