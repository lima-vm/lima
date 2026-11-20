//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package acceptloop

import (
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsRetryable(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EMFILE, syscall.ENFILE, syscall.ENOMEM, syscall.ENOBUFS, syscall.ENOSPC, syscall.ECONNRESET, syscall.ECONNABORTED, syscall.EPROTO} {
		assert.Assert(t, isRetryable(acceptError(errno)), "Serve does not retry errno %d (%v)", errno, errno)
	}
}
