//go:build darwin && arm64

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
)

// nestedVirtualizationEnabled reports whether the host can provide nested virtualization
// for the given accelerator.
//
// On aarch64 macOS, HVF nested virtualization requires QEMU 11.1.0 or later.
// https://wiki.qemu.org/ChangeLog/11.1
func nestedVirtualizationEnabled(accel string, qemuVer *semver.Version) (bool, error) {
	if accel != "hvf" {
		return false, nil
	}
	minVer := semver.New("11.1.0")
	if qemuVer != nil && qemuVer.LessThan(*minVer) {
		return false, fmt.Errorf("QEMU %v is too old, %v or later is required for nested virtualization with HVF", qemuVer, minVer)
	}
	return true, nil
}
