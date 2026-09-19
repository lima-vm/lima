//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestValidateConfigRejectsBlockDevicesOutsideLimactl(t *testing.T) {
	// The default test binary, like lima-driver-vz, is not limactl. No build
	// tag is needed: additional-drivers builds the VZ driver without one.
	err := validateConfig(&limatype.LimaYAML{BlockDevices: []string{"/dev/disk4"}})
	assert.ErrorContains(t, err, "external VZ")
}

func TestGuestBlockDeviceIdentifier(t *testing.T) {
	assert.Equal(t, guestBlockDeviceIdentifier("/dev/disk4"), "disk4")
	assert.Equal(t, guestBlockDeviceIdentifier("/dev/disk@4"), "disk-4")
	assert.Equal(t, guestBlockDeviceIdentifier("/dev/"+strings.Repeat("a", 64)), strings.Repeat("a", 20))
}

func TestRetainedFileDescriptorsArePerVM(t *testing.T) {
	first := newRetainedFileDescriptors()
	second := newRetainedFileDescriptors()

	firstFile, err := os.CreateTemp(t.TempDir(), "first")
	assert.NilError(t, err)
	secondFile, err := os.CreateTemp(t.TempDir(), "second")
	assert.NilError(t, err)

	first.Append(firstFile)
	second.Append(secondFile)
	first.CloseAll()

	_, err = secondFile.Stat()
	assert.NilError(t, err)

	second.CloseAll()
	_, err = secondFile.Stat()
	assert.Assert(t, err != nil)
}

func TestHostBlockDeviceLockAliasesAndRelease(t *testing.T) {
	dir := t.TempDir()
	first, second := newRetainedFileDescriptors(), newRetainedFileDescriptors()
	t.Cleanup(first.CloseAll)
	t.Cleanup(second.CloseAll)
	assert.NilError(t, lockHostBlockDevice(dir, "/dev/disk4", first))
	for _, path := range []string{"/dev/disk4", "/dev/rdisk4", "/dev/disk4s1", "/dev/rdisk4s1s1"} {
		assert.ErrorContains(t, lockHostBlockDevice(dir, path, second), "already in use")
	}
	assert.NilError(t, lockHostBlockDevice(dir, "/dev/disk5", second))
	first.CloseAll()
	assert.NilError(t, lockHostBlockDevice(dir, "/dev/rdisk4s1", second))
}

func TestHostBlockDeviceLockSharedWithinVM(t *testing.T) {
	dir := t.TempDir()
	first, second := newRetainedFileDescriptors(), newRetainedFileDescriptors()
	t.Cleanup(first.CloseAll)
	t.Cleanup(second.CloseAll)
	for _, path := range []string{"/dev/disk4s1", "/dev/disk4s2", "/dev/rdisk4s1", "/dev/disk4", "/dev/disk5"} {
		assert.NilError(t, lockHostBlockDevice(dir, path, first))
		assert.ErrorContains(t, lockHostBlockDevice(dir, path, second), "already in use")
	}
	assert.Equal(t, len(first.files), 2)
	assert.Equal(t, filepath.Base(first.files[0].Name()), "_block-device-disk4.lock")
	first.CloseAll()
	assert.NilError(t, lockHostBlockDevice(dir, "/dev/rdisk4s2", second))
}
