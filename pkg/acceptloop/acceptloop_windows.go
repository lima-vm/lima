// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package acceptloop

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// retryableErrnos are the Accept errors that Serve retries. WSAEMFILE and
// WSAENOBUFS mean that no more sockets can be opened, or that Windows ran
// out of buffer space for a new one. Go's syscall.EMFILE does not match
// Windows' WSAEMFILE. WSAECONNRESET and WSAECONNABORTED mean that one
// incoming connection failed before Serve accepted it.
var retryableErrnos = []syscall.Errno{windows.WSAEMFILE, windows.WSAENOBUFS, windows.WSAECONNRESET, windows.WSAECONNABORTED}
