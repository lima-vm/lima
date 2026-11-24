// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func TestStopCleansUpStalePIDFiles(t *testing.T) {
	instDir := t.TempDir()

	// Write a stale boot session marker
	markerPath := filepath.Join(instDir, filenames.BootSession)
	assert.NilError(t, os.WriteFile(markerPath, []byte("stale-boot-session-id\n"), 0o644))

	// Write a drv.pid file containing current process PID or any dummy PID
	pidFilePath := filepath.Join(instDir, "test.drv.pid")
	assert.NilError(t, os.WriteFile(pidFilePath, []byte("123456\n"), 0o644))

	// Write a drv.sock file
	sockFilePath := filepath.Join(instDir, "test.drv.sock")
	assert.NilError(t, os.WriteFile(sockFilePath, []byte(""), 0o644))

	// Run Stop
	Stop(instDir, false)

	// Verify pidFile and sockFile were cleaned up
	_, err := os.Stat(pidFilePath)
	assert.Assert(t, os.IsNotExist(err), "expected stale pid file to be removed")

	_, err = os.Stat(sockFilePath)
	assert.Assert(t, os.IsNotExist(err), "expected sock file to be removed")
}

func TestStopWithInactivePIDFile(t *testing.T) {
	instDir := t.TempDir()

	// Write current boot PID file for a non-existent PID (e.g. 99999999)
	pidFilePath := filepath.Join(instDir, "test.drv.pid")
	assert.NilError(t, os.WriteFile(pidFilePath, []byte("99999999\n"), 0o644))

	Stop(instDir, false)

	// Since PID 99999999 is not alive, Stop should clean it up
	_, err := os.Stat(pidFilePath)
	assert.Assert(t, os.IsNotExist(err), "expected inactive pid file to be removed")
}

func TestRecordBootSessionCleansStaleFilesAndWritesMarker(t *testing.T) {
	current, err := osutil.BootSessionID()
	if err != nil || current == "" {
		t.Skip("boot session ID not supported on this platform")
	}

	instDir := t.TempDir()
	markerPath := filepath.Join(instDir, filenames.BootSession)
	assert.NilError(t, os.WriteFile(markerPath, []byte("stale-boot-session-id\n"), 0o644))

	stalePID := filepath.Join(instDir, "other.drv.pid")
	staleSock := filepath.Join(instDir, "other.drv.sock")
	assert.NilError(t, os.WriteFile(stalePID, []byte("123456\n"), 0o644))
	assert.NilError(t, os.WriteFile(staleSock, []byte(""), 0o644))

	assert.NilError(t, recordBootSession(instDir))

	_, err = os.Stat(stalePID)
	assert.Assert(t, os.IsNotExist(err), "expected stale pid file to be cleaned before refreshing marker")
	_, err = os.Stat(staleSock)
	assert.Assert(t, os.IsNotExist(err), "expected stale sock file to be cleaned before refreshing marker")

	got, err := os.ReadFile(markerPath)
	assert.NilError(t, err)
	assert.Equal(t, current, strings.TrimSpace(string(got)))
}
