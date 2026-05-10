//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGuestBlockDeviceIdentifier(t *testing.T) {
	assert.Equal(t, guestBlockDeviceIdentifier("/dev/disk4"), "disk4")
	assert.Equal(t, guestBlockDeviceIdentifier("/dev/disk@4"), "disk-4")
	assert.Equal(t, guestBlockDeviceIdentifier("/dev/"+strings.Repeat("a", 64)), strings.Repeat("a", 20))
}
