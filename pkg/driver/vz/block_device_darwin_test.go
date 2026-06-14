//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"os"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

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
