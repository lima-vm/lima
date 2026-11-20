// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestValidateSnapshotTag(t *testing.T) {
	tests := []struct {
		tag     string
		wantErr bool
	}{
		{tag: "base"},
		{tag: "snap-1"},
		{tag: "", wantErr: true},
		{tag: ".", wantErr: true},
		{tag: "..", wantErr: true},
		{tag: "../evil", wantErr: true},
		{tag: "a/b", wantErr: true},
		{tag: "/abs", wantErr: true},
		{tag: `a\b`, wantErr: true},
		{tag: "a b", wantErr: true},
		{tag: "a\tb", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.tag), func(t *testing.T) {
			err := validateSnapshotTag(tt.tag)
			if tt.wantErr {
				assert.ErrorContains(t, err, "invalid snapshot tag")
				return
			}
			assert.NilError(t, err)
		})
	}
}

// stoppedInstance returns a stopped instance whose boot disk contains "v1".
func stoppedInstance(t *testing.T) *limatype.Instance {
	t.Helper()
	inst := &limatype.Instance{
		Dir:    t.TempDir(),
		Status: limatype.StatusStopped,
		Config: &limatype.LimaYAML{VMType: new(limatype.VZ)},
	}
	assert.NilError(t, os.WriteFile(filepath.Join(inst.Dir, filenames.Disk), []byte("v1"), 0o644))
	return inst
}

func TestDiskSnapshot(t *testing.T) {
	inst := stoppedInstance(t)
	disk := filepath.Join(inst.Dir, filenames.Disk)

	snapshots, err := ListDiskSnapshots(inst)
	assert.NilError(t, err)
	assert.Equal(t, len(snapshots), 0)

	assert.NilError(t, CreateDiskSnapshot(inst, "base"))
	assert.ErrorContains(t, CreateDiskSnapshot(inst, "base"), "already exists")
	snapshots, err = ListDiskSnapshots(inst)
	assert.NilError(t, err)
	assert.DeepEqual(t, snapshots, []driver.Snapshot{{ID: "base", Tag: "base"}})

	// Writing to the disk must not change the snapshot, and applying it must restore the old content.
	assert.NilError(t, os.WriteFile(disk, []byte("v2"), 0o644))
	assert.NilError(t, ApplyDiskSnapshot(inst, "base"))
	b, err := os.ReadFile(disk)
	assert.NilError(t, err)
	assert.Equal(t, string(b), "v1")

	assert.NilError(t, DeleteDiskSnapshot(inst, "base"))
	snapshots, err = ListDiskSnapshots(inst)
	assert.NilError(t, err)
	assert.Equal(t, len(snapshots), 0)
	assert.ErrorContains(t, ApplyDiskSnapshot(inst, "base"), `snapshot "base"`)
	assert.ErrorContains(t, DeleteDiskSnapshot(inst, "base"), `snapshot "base"`)
}

func TestDiskSnapshotRequiresStoppedInstance(t *testing.T) {
	inst := stoppedInstance(t)
	assert.NilError(t, CreateDiskSnapshot(inst, "base"))

	inst.Status = limatype.StatusRunning
	const want = `vz disk snapshots require a stopped instance (status "Running")`
	assert.ErrorContains(t, CreateDiskSnapshot(inst, "other"), want)
	assert.ErrorContains(t, ApplyDiskSnapshot(inst, "base"), want)
	assert.ErrorContains(t, DeleteDiskSnapshot(inst, "base"), want)

	// Listing works on a running instance.
	snapshots, err := ListDiskSnapshots(inst)
	assert.NilError(t, err)
	assert.Equal(t, len(snapshots), 1)
}

func TestDiskSnapshotExtraFiles(t *testing.T) {
	inst := stoppedInstance(t)
	write := func(name, content string) {
		t.Helper()
		assert.NilError(t, os.WriteFile(filepath.Join(inst.Dir, name), []byte(content), 0o644))
	}
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(inst.Dir, name))
		assert.NilError(t, err)
		return string(b)
	}

	// "aux" does not exist yet, so the snapshot does not have it.
	write("efi", "e1")
	assert.NilError(t, CreateDiskSnapshot(inst, "base", "efi", "aux"))
	_, err := os.Stat(filepath.Join(inst.Dir, snapshotsDirName, "base", "aux"))
	assert.Assert(t, errors.Is(err, os.ErrNotExist))

	write(filenames.Disk, "v2")
	write("efi", "e2")
	write("aux", "a2")
	assert.NilError(t, ApplyDiskSnapshot(inst, "base", "efi", "aux"))
	assert.Equal(t, read(filenames.Disk), "v1")
	assert.Equal(t, read("efi"), "e1")
	assert.Equal(t, read("aux"), "a2")
}

func TestApplyDiskSnapshotKeepsDiskSymlink(t *testing.T) {
	// In instances created by older versions of Lima, `disk` is a symlink to `diffdisk`.
	inst := stoppedInstance(t)
	disk := filepath.Join(inst.Dir, filenames.Disk)
	diffDisk := filepath.Join(inst.Dir, filenames.DiffDiskLegacy)
	assert.NilError(t, os.Rename(disk, diffDisk))
	assert.NilError(t, os.Symlink(filenames.DiffDiskLegacy, disk))

	assert.NilError(t, CreateDiskSnapshot(inst, "base"))
	fi, err := os.Lstat(filepath.Join(inst.Dir, snapshotsDirName, "base", filenames.Disk))
	assert.NilError(t, err)
	assert.Assert(t, fi.Mode().IsRegular(), "the snapshot must have a copy of the disk, not of the symlink")

	assert.NilError(t, os.WriteFile(disk, []byte("v2"), 0o644))
	assert.NilError(t, ApplyDiskSnapshot(inst, "base"))
	target, err := os.Readlink(disk)
	assert.NilError(t, err)
	assert.Equal(t, target, filenames.DiffDiskLegacy)
	b, err := os.ReadFile(diffDisk)
	assert.NilError(t, err)
	assert.Equal(t, string(b), "v1")
	leftovers, err := filepath.Glob(filepath.Join(inst.Dir, "*.snap-tmp"))
	assert.NilError(t, err)
	assert.Equal(t, len(leftovers), 0)
}
