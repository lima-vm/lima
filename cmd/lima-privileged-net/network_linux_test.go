// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"

	"gotest.tools/v3/assert"
)

func TestStop(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "test.pid")
	setPID := func(t *testing.T, pid int) {
		t.Helper()
		assert.NilError(t, os.WriteFile(pidFile, fmt.Appendf(nil, "%d\n", pid), 0o644))
	}

	t.Run("same executable", func(t *testing.T) {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM)
		defer signal.Stop(sig)
		setPID(t, os.Getpid())
		assert.NilError(t, stop(pidFile))
		assert.Equal(t, <-sig, os.Signal(syscall.SIGTERM))
	})

	t.Run("PID reused by another executable", func(t *testing.T) {
		cmd := exec.CommandContext(t.Context(), "sleep", "60")
		assert.NilError(t, cmd.Start())
		setPID(t, cmd.Process.Pid)
		// A `start` that holds the lock is about to replace the file.
		lock, err := lockFile(pidFile + ".lock")
		assert.NilError(t, err)
		assert.ErrorContains(t, stop(pidFile), "is stale")
		assert.NilError(t, lock.Close())
		// Without one, the `start` that wrote the file is gone, and so is the file.
		assert.NilError(t, stop(pidFile))
		_, err = os.Stat(pidFile)
		assert.Assert(t, os.IsNotExist(err))
		assert.NilError(t, cmd.Process.Signal(syscall.Signal(0)))
		assert.NilError(t, cmd.Process.Kill())
		assert.ErrorContains(t, cmd.Wait(), "signal: killed") // rather than "terminated"

		// The PID is free now.
		setPID(t, cmd.Process.Pid)
		assert.NilError(t, stop(pidFile))
	})

	t.Run("second start", func(t *testing.T) {
		lockPath := pidFile + ".lock"
		lock, err := lockFile(lockPath)
		assert.NilError(t, err)
		_, err = lockFile(lockPath)
		assert.ErrorContains(t, err, "already up")
		assert.NilError(t, lock.Close())
		lock, err = lockFile(lockPath)
		assert.NilError(t, err)
		assert.NilError(t, lock.Close())
	})

	t.Run("PID file write", func(t *testing.T) {
		assert.NilError(t, writePIDFile(pidFile))
		b, err := os.ReadFile(pidFile)
		assert.NilError(t, err)
		assert.Equal(t, string(b), fmt.Sprintf("%d\n", os.Getpid()))
		_, err = os.Stat(pidFile + ".tmp")
		assert.Assert(t, os.IsNotExist(err))
	})

	t.Run("not a PID", func(t *testing.T) {
		// A negative number would address a process group.
		for _, content := range []string{"", "x", "0", "-1"} {
			assert.NilError(t, os.WriteFile(pidFile, []byte(content+"\n"), 0o644))
			assert.ErrorContains(t, stop(pidFile), "invalid PID")
		}
	})
}
