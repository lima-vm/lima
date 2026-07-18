// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package portfwd

import (
	"syscall"

	"golang.org/x/sys/windows"
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
	return err
}
