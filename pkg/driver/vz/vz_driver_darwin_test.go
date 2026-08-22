//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func TestWaitForStopped(t *testing.T) {
	t.Run("returns nil after the VM has stopped", func(t *testing.T) {
		l := &LimaVzDriver{machine: &virtualMachineWrapper{}}
		go func() {
			l.machine.mu.Lock()
			l.machine.stopped = true
			l.machine.mu.Unlock()
		}()
		assert.NilError(t, l.waitForStopped(10*time.Second))
	})

	t.Run("returns an error when the VM is still running", func(t *testing.T) {
		l := &LimaVzDriver{machine: &virtualMachineWrapper{}}
		assert.ErrorContains(t, l.waitForStopped(600*time.Millisecond), "timeout")
	})
}

func TestRemovePIDFile(t *testing.T) {
	dir := t.TempDir()
	l := &LimaVzDriver{
		Instance: &limatype.Instance{
			Dir:    dir,
			Config: &limatype.LimaYAML{VMType: ptr.Of(limatype.VZ)},
		},
	}
	pidFile := filepath.Join(dir, filenames.PIDFile(limatype.VZ))
	assert.NilError(t, os.WriteFile(pidFile, []byte("42\n"), 0o644))

	l.removePIDFile()
	_, err := os.Stat(pidFile)
	assert.Assert(t, errors.Is(err, os.ErrNotExist))

	// Removing a PID file that does not exist is not an error.
	l.removePIDFile()
}
