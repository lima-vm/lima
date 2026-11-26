// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func TestDiskDeleteActionBatchErrorAccumulation(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)

	inst1Dir := filepath.Join(limaHome, "inst1")
	assert.NilError(t, os.MkdirAll(inst1Dir, 0o755))
	disk1Dir, err := store.DiskDir("disk1")
	assert.NilError(t, err)
	assert.NilError(t, os.MkdirAll(disk1Dir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(disk1Dir, filenames.DataDisk), make([]byte, 1024), 0o644))
	assert.NilError(t, os.Symlink(inst1Dir, filepath.Join(disk1Dir, filenames.InUseBy)))

	inst2Dir := filepath.Join(limaHome, "inst2")
	assert.NilError(t, os.MkdirAll(inst2Dir, 0o755))
	disk2Dir, err := store.DiskDir("disk2")
	assert.NilError(t, err)
	assert.NilError(t, os.MkdirAll(disk2Dir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(disk2Dir, filenames.DataDisk), make([]byte, 1024), 0o644))
	assert.NilError(t, os.Symlink(inst2Dir, filepath.Join(disk2Dir, filenames.InUseBy)))

	cmd := newTestDiskDeleteCommand(t)

	err = diskDeleteAction(cmd, []string{"disk1", "disk2"})
	assert.Assert(t, err != nil, "diskDeleteAction should return accumulated errors")
	assert.ErrorContains(t, err, "disk1")
	assert.ErrorContains(t, err, "disk2")
}

func newTestDiskDeleteCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().BoolP("force", "f", false, "")
	cmd.SetContext(t.Context())
	return cmd
}

func createTestDisk(t *testing.T, name, inUseByInstanceDir string) string {
	t.Helper()
	diskDir, err := store.DiskDir(name)
	assert.NilError(t, err)
	assert.NilError(t, os.MkdirAll(diskDir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(diskDir, filenames.DataDisk), make([]byte, 1024), 0o644))
	if inUseByInstanceDir != "" {
		assert.NilError(t, os.Symlink(inUseByInstanceDir, filepath.Join(diskDir, filenames.InUseBy)))
	}
	return diskDir
}

func TestDiskDeleteActionBatchMixedSuccessAndFailure(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)

	instDir := filepath.Join(limaHome, "inst1")
	assert.NilError(t, os.MkdirAll(instDir, 0o755))

	lockedDir := createTestDisk(t, "locked1", instDir)
	ok1Dir := createTestDisk(t, "ok1", "")
	ok2Dir := createTestDisk(t, "ok2", "")

	err := diskDeleteAction(newTestDiskDeleteCommand(t), []string{"locked1", "ok1", "../bad", "ok2", "nonexistent"})
	assert.Assert(t, err != nil, "diskDeleteAction should return accumulated errors")
	assert.ErrorContains(t, err, "locked1")
	assert.ErrorContains(t, err, "../bad")

	for _, removed := range []string{ok1Dir, ok2Dir} {
		_, statErr := os.Stat(removed)
		assert.Assert(t, os.IsNotExist(statErr), "%s should have been deleted", removed)
	}
	_, statErr := os.Stat(lockedDir)
	assert.NilError(t, statErr, "locked disk directory should still exist")
}

func TestDiskDeleteActionSuccessAndSingleFailure(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)

	instDir := filepath.Join(limaHome, "inst1")
	assert.NilError(t, os.MkdirAll(instDir, 0o755))

	ok1Dir := createTestDisk(t, "ok1", "")
	ok2Dir := createTestDisk(t, "ok2", "")
	lockedDir := createTestDisk(t, "locked1", instDir)

	assert.NilError(t, diskDeleteAction(newTestDiskDeleteCommand(t), []string{"ok1", "ok2"}))
	for _, removed := range []string{ok1Dir, ok2Dir} {
		_, statErr := os.Stat(removed)
		assert.Assert(t, os.IsNotExist(statErr), "%s should have been deleted", removed)
	}

	err := diskDeleteAction(newTestDiskDeleteCommand(t), []string{"locked1"})
	assert.ErrorContains(t, err, "locked1")
	_, statErr := os.Stat(lockedDir)
	assert.NilError(t, statErr, "locked disk directory should still exist")
}
