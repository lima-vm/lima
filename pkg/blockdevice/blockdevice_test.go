// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGuestDeviceIdentifier(t *testing.T) {
	assert.Equal(t, GuestDeviceIdentifier("/dev/disk4"), "disk4")
	assert.Equal(t, GuestDeviceIdentifier("/dev/disk@4"), "disk-4")
	assert.Equal(t, GuestDeviceIdentifier("/dev/"+strings.Repeat("a", 64)), strings.Repeat("a", 20))
	// Windows DOS device paths, in both the native and the MSYS2-safe form.
	assert.Equal(t, GuestDeviceIdentifier(`\\.\PhysicalDrive2`), "PhysicalDrive2")
	assert.Equal(t, GuestDeviceIdentifier("//./PhysicalDrive2"), "PhysicalDrive2")
}
