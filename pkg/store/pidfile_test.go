// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// fakeBootSessionID replaces the source of the boot session ID for the duration of the test.
func fakeBootSessionID(t *testing.T, id string, err error) {
	t.Helper()
	orig := bootSessionID
	bootSessionID = func() (string, error) { return id, err }
	t.Cleanup(func() { bootSessionID = orig })
}

func TestWritePIDFile(t *testing.T) {
	fakeBootSessionID(t, "boot-1", nil)
	dir := t.TempDir()
	path := filepath.Join(dir, filenames.HostAgentPID)
	assert.NilError(t, WritePIDFile(path, os.Getpid()))

	assert.Assert(t, osutil.FileExists(filepath.Join(dir, filenames.BootSession)))

	pid, err := ReadPIDFile(path)
	assert.NilError(t, err)
	assert.Equal(t, pid, os.Getpid())
}

func TestWritePIDFileRemovesPIDFilesFromPreviousBoot(t *testing.T) {
	fakeBootSessionID(t, "boot-1", nil)
	dir := t.TempDir()
	haPath := filepath.Join(dir, filenames.HostAgentPID)
	driverPath := filepath.Join(dir, filenames.PIDFile(limatype.QEMU))
	assert.NilError(t, WritePIDFile(haPath, os.Getpid()))
	assert.NilError(t, WritePIDFile(driverPath, os.Getpid()))

	// Writing a PID file during a new boot voids every PID file of the directory, so that
	// refreshing the marker cannot make a PID file of the previous boot look current.
	fakeBootSessionID(t, "boot-2", nil)
	assert.NilError(t, WritePIDFile(haPath, os.Getpid()))

	_, err := os.Stat(driverPath)
	assert.Assert(t, errors.Is(err, os.ErrNotExist))

	pid, err := ReadPIDFile(haPath)
	assert.NilError(t, err)
	assert.Equal(t, pid, os.Getpid())
}

func TestReadPIDFileFromPreviousBoot(t *testing.T) {
	fakeBootSessionID(t, "boot-1", nil)
	dir := t.TempDir()
	path := filepath.Join(dir, filenames.HostAgentPID)
	assert.NilError(t, WritePIDFile(path, os.Getpid()))

	// After a reboot the PID may belong to an unrelated process, so the PID file has to be
	// ignored even though the PID is alive.
	fakeBootSessionID(t, "boot-2", nil)

	pid, err := ReadPIDFile(path)
	assert.NilError(t, err)
	assert.Equal(t, pid, 0)
	// Removing it is left to the next writer, so that a PID file that another process is
	// writing at this very moment cannot be removed by a reader.
	assert.Assert(t, osutil.FileExists(path))
}

func TestReadPIDFileCorruptedFromPreviousBoot(t *testing.T) {
	fakeBootSessionID(t, "boot-1", nil)
	dir := t.TempDir()
	path := filepath.Join(dir, filenames.HostAgentPID)
	assert.NilError(t, WritePIDFile(path, os.Getpid()))
	// A PID file that was truncated when the host was powered off.
	assert.NilError(t, os.WriteFile(path, nil, 0o644))

	fakeBootSessionID(t, "boot-2", nil)

	pid, err := ReadPIDFile(path)
	assert.NilError(t, err)
	assert.Equal(t, pid, 0)
}

func TestReadPIDFileWithoutBootSessionMarker(t *testing.T) {
	// PID files written by an older version of Lima have no marker, and are handled as
	// before: only the liveness of the process matters.
	fakeBootSessionID(t, "boot-1", nil)
	dir := t.TempDir()
	path := filepath.Join(dir, filenames.HostAgentPID)
	assert.NilError(t, os.WriteFile(path, fmt.Appendf(nil, "%d\n", os.Getpid()), 0o644))

	pid, err := ReadPIDFile(path)
	assert.NilError(t, err)
	assert.Equal(t, pid, os.Getpid())
}

func TestWritePIDFileWhenBootSessionIDFails(t *testing.T) {
	// The PID file must not be written when it cannot be tied to the current boot:
	// a marker of a previous boot would otherwise void a PID file that is current.
	fakeBootSessionID(t, "", errors.New("sysctl failed"))
	dir := t.TempDir()
	path := filepath.Join(dir, filenames.HostAgentPID)

	assert.ErrorContains(t, WritePIDFile(path, os.Getpid()), "sysctl failed")
	assert.Assert(t, !osutil.FileExists(path))
}

func TestReadPIDFileWhenBootSessionIDFails(t *testing.T) {
	fakeBootSessionID(t, "boot-1", nil)
	dir := t.TempDir()
	path := filepath.Join(dir, filenames.HostAgentPID)
	assert.NilError(t, WritePIDFile(path, os.Getpid()))

	fakeBootSessionID(t, "", errors.New("sysctl failed"))
	_, err := ReadPIDFile(path)
	assert.ErrorContains(t, err, "sysctl failed")
}

func TestWritePIDFileWithoutBootSessionSupport(t *testing.T) {
	fakeBootSessionID(t, "", osutil.ErrBootSessionNotSupported)
	dir := t.TempDir()
	path := filepath.Join(dir, filenames.HostAgentPID)
	assert.NilError(t, WritePIDFile(path, os.Getpid()))

	assert.Assert(t, !osutil.FileExists(filepath.Join(dir, filenames.BootSession)))

	pid, err := ReadPIDFile(path)
	assert.NilError(t, err)
	assert.Equal(t, pid, os.Getpid())
}
