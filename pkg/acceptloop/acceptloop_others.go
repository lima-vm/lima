//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package acceptloop

import "syscall"

// outOfResourceErrnos are the Accept errors that mean the process or the
// system ran out of file descriptors or memory. On Linux, Accept also fails
// with ENOSPC when Go cannot register the new connection with epoll,
// because the user that runs the process has reached the limit in
// /proc/sys/fs/epoll/max_user_watches.
var outOfResourceErrnos = []syscall.Errno{syscall.EMFILE, syscall.ENFILE, syscall.ENOMEM, syscall.ENOBUFS, syscall.ENOSPC}
