// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package acceptloop

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// outOfResourceErrnos are the Accept errors that mean that no more sockets
// can be opened, or that Windows ran out of buffer space for a new one. Go's
// syscall.EMFILE does not match Windows' WSAEMFILE.
var outOfResourceErrnos = []syscall.Errno{windows.WSAEMFILE, windows.WSAENOBUFS}
