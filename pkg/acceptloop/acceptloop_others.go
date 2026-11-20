//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package acceptloop

import "syscall"

// retryableErrnos are the Accept errors that Serve retries. EMFILE, ENFILE,
// ENOMEM and ENOBUFS mean that the process or the system ran out of file
// descriptors or memory. On Linux, Accept also fails with ENOSPC when Go
// cannot register the new connection with epoll, because the user that runs
// the process has reached the limit in /proc/sys/fs/epoll/max_user_watches.
// ECONNRESET, ECONNABORTED and EPROTO mean that one incoming connection
// failed before Serve accepted it.
var retryableErrnos = []syscall.Errno{syscall.EMFILE, syscall.ENFILE, syscall.ENOMEM, syscall.ENOBUFS, syscall.ENOSPC, syscall.ECONNRESET, syscall.ECONNABORTED, syscall.EPROTO}
