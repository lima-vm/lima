// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

// writeMinimalRawDisk writes an empty raw disk file that store.InspectDisk can
// inspect.
func writeMinimalRawDisk(t *testing.T, path string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	assert.NilError(t, os.WriteFile(path, make([]byte, 64*1024), 0o644))
}

func TestDiskCreateWithDiskPath(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())

	// `--disk-path` takes the path of the disk file, not of its directory.
	realDataDisk := filepath.Join(t.TempDir(), "nested", "testdisk.img")
	cmd := newDiskCreateCommand()
	cmd.SetContext(t.Context())
	assert.NilError(t, cmd.ParseFlags([]string{"--size", "1MiB", "--format", "raw", "--disk-path", realDataDisk}))
	assert.NilError(t, diskCreateAction(cmd, []string{"testdisk"}))

	diskDir, err := store.DiskDir("testdisk")
	assert.NilError(t, err)

	// The disk directory in $LIMA_HOME must stay a regular directory; only the
	// data disk file is relocated and symlinked.
	assert.Assert(t, !osutil.IsSymlink(diskDir), "%q must not be a symlink itself", diskDir)
	fi, err := os.Stat(diskDir)
	assert.NilError(t, err)
	assert.Assert(t, fi.IsDir(), "%q must be a directory", diskDir)

	dataDisk := filepath.Join(diskDir, filenames.DataDisk)
	assert.Assert(t, osutil.IsSymlink(dataDisk), "%q must be a symlink", dataDisk)
	wantTarget, err := filepath.Abs(realDataDisk)
	assert.NilError(t, err)
	target, err := os.Readlink(dataDisk)
	assert.NilError(t, err)
	assert.Equal(t, target, wantTarget)
	assert.Assert(t, osutil.FileExists(wantTarget), "the data disk must be created at %q", wantTarget)
}

func TestDiskCreateRejectsDirectoryDiskPath(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())

	cmd := newDiskCreateCommand()
	cmd.SetContext(t.Context())
	assert.NilError(t, cmd.ParseFlags([]string{"--size", "1MiB", "--format", "raw", "--disk-path", t.TempDir()}))

	err := diskCreateAction(cmd, []string{"testdisk"})
	assert.ErrorContains(t, err, "is a directory")

	diskDir, err := store.DiskDir("testdisk")
	assert.NilError(t, err)
	_, err = os.Lstat(diskDir)
	assert.Assert(t, os.IsNotExist(err), "the disk must not be created")
}

// Without `--disk-path` the data disk has to stay in the disk directory.
func TestDiskCreateDefaultPath(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())

	cmd := newDiskCreateCommand()
	cmd.SetContext(t.Context())
	assert.NilError(t, cmd.ParseFlags([]string{"--size", "1MiB", "--format", "raw"}))
	assert.NilError(t, diskCreateAction(cmd, []string{"testdisk"}))

	diskDir, err := store.DiskDir("testdisk")
	assert.NilError(t, err)
	dataDisk := filepath.Join(diskDir, filenames.DataDisk)
	assert.Assert(t, !osutil.IsSymlink(dataDisk), "%q must not be a symlink", dataDisk)
	assert.Assert(t, osutil.FileExists(dataDisk), "the data disk must be created at %q", dataDisk)
}

func TestDiskCreateRejectsExistingDiskPath(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())

	diskPath := filepath.Join(t.TempDir(), "existing.img")
	writeMinimalRawDisk(t, diskPath)

	cmd := newDiskCreateCommand()
	cmd.SetContext(t.Context())
	assert.NilError(t, cmd.ParseFlags([]string{"--size", "1MiB", "--disk-path", diskPath}))

	err := diskCreateAction(cmd, []string{"testdisk"})
	assert.ErrorContains(t, err, "already exists")

	diskDir, err := store.DiskDir("testdisk")
	assert.NilError(t, err)
	_, err = os.Lstat(diskDir)
	assert.Assert(t, os.IsNotExist(err), "the disk must not be created")
}

// A relative `--disk-path` is resolved against the current working directory.
func TestDiskCreateWithRelativeDiskPath(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	workDir := t.TempDir()
	t.Chdir(workDir)

	cmd := newDiskCreateCommand()
	cmd.SetContext(t.Context())
	assert.NilError(t, cmd.ParseFlags([]string{"--size", "1MiB", "--disk-path", filepath.Join("disks", "testdisk.img")}))
	assert.NilError(t, diskCreateAction(cmd, []string{"testdisk"}))

	diskDir, err := store.DiskDir("testdisk")
	assert.NilError(t, err)
	target, err := os.Readlink(filepath.Join(diskDir, filenames.DataDisk))
	assert.NilError(t, err)
	wantTarget := filepath.Join(workDir, "disks", "testdisk.img")
	assert.Equal(t, target, wantTarget)
	assert.Assert(t, osutil.FileExists(wantTarget), "the data disk must be created at %q", wantTarget)
}

func TestDeleteDisk(t *testing.T) {
	t.Run("regular disk directory", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		diskDir, err := store.DiskDir("testdisk")
		assert.NilError(t, err)
		writeMinimalRawDisk(t, filepath.Join(diskDir, filenames.DataDisk))

		assert.NilError(t, deleteDisk(&store.Disk{Name: "testdisk", Dir: diskDir}))

		assert.Assert(t, !osutil.FileExists(diskDir), "the disk directory must be removed")
	})

	t.Run("data disk symlinked outside of the disk directory", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		diskDir, err := store.DiskDir("testdisk")
		assert.NilError(t, err)
		realDiskDir := t.TempDir()
		realDataDisk := filepath.Join(realDiskDir, "testdisk.img")
		writeMinimalRawDisk(t, realDataDisk)
		assert.NilError(t, os.MkdirAll(diskDir, 0o700))
		assert.NilError(t, os.Symlink(realDataDisk, filepath.Join(diskDir, filenames.DataDisk)))

		assert.NilError(t, deleteDisk(&store.Disk{Name: "testdisk", Dir: diskDir}))

		assert.Assert(t, !osutil.FileExists(realDataDisk),
			"the data disk behind the symlink must be removed, not just the symlink")
		assert.Assert(t, !osutil.FileExists(diskDir), "the disk directory must be removed")
		// Only the disk file is removed from the custom directory.
		assert.Assert(t, osutil.FileExists(realDiskDir), "%q must be kept", realDiskDir)
	})

	t.Run("legacy symlinked disk directory", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		diskDir, err := store.DiskDir("legacydisk")
		assert.NilError(t, err)
		realDiskDir := t.TempDir()
		writeMinimalRawDisk(t, filepath.Join(realDiskDir, filenames.DataDisk))
		assert.NilError(t, os.MkdirAll(filepath.Dir(diskDir), 0o700))
		assert.NilError(t, os.Symlink(realDiskDir, diskDir))

		assert.NilError(t, deleteDisk(&store.Disk{Name: "legacydisk", Dir: diskDir}))

		assert.Assert(t, !osutil.FileExists(realDiskDir), "the symlink target must be removed")
		_, err = os.Lstat(diskDir)
		assert.Assert(t, os.IsNotExist(err), "the symlink itself must be removed")
	})
}

func TestDiskListShowsRealDiskDirectory(t *testing.T) {
	tests := []struct {
		name string
		// prepare creates the disk "testdisk" in diskDir and returns the
		// directory that is expected in the DIR column.
		prepare func(t *testing.T, diskDir string) string
	}{
		{
			name: "data disk symlinked outside of the disk directory",
			prepare: func(t *testing.T, diskDir string) string {
				t.Helper()
				realDiskDir := t.TempDir()
				realDataDisk := filepath.Join(realDiskDir, "testdisk.img")
				writeMinimalRawDisk(t, realDataDisk)
				assert.NilError(t, os.MkdirAll(diskDir, 0o700))
				assert.NilError(t, os.Symlink(realDataDisk, filepath.Join(diskDir, filenames.DataDisk)))
				return realDiskDir
			},
		},
		{
			name: "legacy symlinked disk directory",
			prepare: func(t *testing.T, diskDir string) string {
				t.Helper()
				realDiskDir := t.TempDir()
				writeMinimalRawDisk(t, filepath.Join(realDiskDir, filenames.DataDisk))
				assert.NilError(t, os.MkdirAll(filepath.Dir(diskDir), 0o700))
				assert.NilError(t, os.Symlink(realDiskDir, diskDir))
				return realDiskDir
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LIMA_HOME", t.TempDir())
			diskDir, err := store.DiskDir("testdisk")
			assert.NilError(t, err)
			wantDir := tt.prepare(t, diskDir)

			buf := &bytes.Buffer{}
			cmd := newDiskListCommand()
			cmd.SetOut(buf)
			assert.NilError(t, diskListAction(cmd, []string{"testdisk"}))

			dir, ok := diskListDirColumn(buf.String(), "testdisk")
			assert.Assert(t, ok, "no row for %q in %q", "testdisk", buf.String())
			assert.Equal(t, dir, wantDir)
		})
	}
}

// diskListDirColumn returns the DIR column of the row for diskName.
func diskListDirColumn(out, diskName string) (string, bool) {
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == diskName {
			return fields[3], true
		}
	}
	return "", false
}
