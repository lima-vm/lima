// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

const (
	statusHeader = "hub port sta spd dev      sockfd local_busid\n"
	free0        = "hs  0000 004 000 00000000 000000 0-0\n"
	free1        = "hs  0001 004 000 00000000 000000 0-0\n"
	wait0        = "hs  0000 005 000 00000000 000000 0-0\n"
	wait1        = "hs  0001 005 000 00000000 000000 0-0\n"
	used0        = "hs  0000 006 002 00010002 000003 3-1\n"
	used1        = "hs  0001 006 002 00010003 000004 3-2\n"
	ssFree       = "ss  0008 004 000 00000000 000000 0-0\n"
)

func fakeSysfs(t *testing.T, status string, write func(path, data string) error) *[]string {
	t.Helper()
	oldDir, oldDevices, oldWrite := vhciDir, usbDevicesPath, writeSysfs
	t.Cleanup(func() { vhciDir, usbDevicesPath, writeSysfs = oldDir, oldDevices, oldWrite })
	vhciDir, usbDevicesPath = t.TempDir(), t.TempDir()
	setStatus(t, status)
	var writes []string
	writeSysfs = func(path, data string) error {
		writes = append(writes, filepath.Base(path)+" "+data)
		if write != nil {
			return write(path, data)
		}
		return nil
	}
	return &writes
}

func setStatus(t *testing.T, status string) {
	t.Helper()
	assert.NilError(t, os.WriteFile(filepath.Join(vhciDir, "status"), []byte(statusHeader+status), 0o644))
}

func TestAttachPortRetriesOnEBUSY(t *testing.T) {
	var writes *[]string
	writes = fakeSysfs(t, free0+free1, func(_, _ string) error {
		if len(*writes) == 1 {
			setStatus(t, used0+free1)
			return &os.PathError{Op: "write", Path: "attach", Err: syscall.EBUSY}
		}
		return nil
	})
	port, err := attachPort(9, 7, usbipSpeedHigh)
	assert.NilError(t, err)
	assert.Equal(t, port, 1)
	assert.DeepEqual(t, *writes, []string{"attach 0 9 7 3", "attach 1 9 7 3"})
}

func TestAttachPortStops(t *testing.T) {
	t.Run("other error", func(t *testing.T) {
		writes := fakeSysfs(t, free0+free1, func(_, _ string) error { return errors.New("bad fd") })
		_, err := attachPort(9, 7, usbipSpeedHigh)
		assert.ErrorContains(t, err, "bad fd")
		assert.Equal(t, len(*writes), 1)
	})
	t.Run("EBUSY until no port is left", func(t *testing.T) {
		fakeSysfs(t, free0, func(_, _ string) error {
			setStatus(t, used0)
			return syscall.EBUSY
		})
		_, err := attachPort(9, 7, usbipSpeedHigh)
		assert.ErrorContains(t, err, "no free vhci hs port")
	})
	t.Run("superspeed uses its own hub", func(t *testing.T) {
		writes := fakeSysfs(t, free0+ssFree, nil)
		port, err := attachPort(9, 7, usbipSpeedSuper)
		assert.NilError(t, err)
		assert.Equal(t, port, 8)
		assert.DeepEqual(t, *writes, []string{"attach 8 9 7 5"})
	})
}

func TestWaitEnumerated(t *testing.T) {
	const short = 200 * time.Millisecond
	addDevice := func(t *testing.T, name string) {
		t.Helper()
		assert.NilError(t, os.Mkdir(filepath.Join(usbDevicesPath, name), 0o755))
	}

	t.Run("enumerates late", func(t *testing.T) {
		writes := fakeSysfs(t, wait0, nil)
		addDevice(t, "3-1")
		timer := time.AfterFunc(150*time.Millisecond, func() { setStatus(t, used0) })
		defer timer.Stop()
		assert.NilError(t, waitEnumerated(0, "1-1", 5*time.Second))
		assert.Equal(t, len(*writes), 0)
	})
	t.Run("used with no sysfs device", func(t *testing.T) {
		writes := fakeSysfs(t, used0, nil)
		assert.ErrorContains(t, waitEnumerated(0, "1-1", short), "did not enumerate")
		assert.DeepEqual(t, *writes, []string{"detach 0"})
	})
	t.Run("another port is healthy", func(t *testing.T) {
		writes := fakeSysfs(t, used0+wait1, nil)
		addDevice(t, "3-1")
		err := waitEnumerated(1, "1-1", short)
		assert.Assert(t, err != nil && strings.Contains(err.Error(), "port 1"), "%v", err)
		assert.DeepEqual(t, *writes, []string{"detach 1"})
	})
	t.Run("status unreadable", func(t *testing.T) {
		writes := fakeSysfs(t, used1, nil)
		assert.NilError(t, os.Remove(filepath.Join(vhciDir, "status")))
		assert.ErrorContains(t, waitEnumerated(1, "1-1", short), "did not enumerate")
		assert.DeepEqual(t, *writes, []string{"detach 1"})
	})
}
