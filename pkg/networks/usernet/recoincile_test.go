// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestWaitForSock_Success(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "test.sock")

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.WriteFile(sockPath, []byte("ok"), 0o644)
	}()

	err := waitForSock(t.Context(), sockPath, 2*time.Second)
	assert.NilError(t, err)
}

func TestWaitForSock_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "nonexistent.sock")

	err := waitForSock(t.Context(), sockPath, 150*time.Millisecond)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "timed out waiting for socket"))
}

func TestWaitForSock_ContextCanceled(t *testing.T) {
	tmpDir := t.TempDir()
	sockPath := filepath.Join(tmpDir, "nonexistent.sock")

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := waitForSock(ctx, sockPath, 2*time.Second)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "context canceled while waiting for socket"))
}
