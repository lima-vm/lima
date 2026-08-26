//go:build linux && arm64

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
// On aarch64 Linux, KVM_CAP_ARM_EL2 requires QEMU 10.1.0 or later and a host kernel
// (6.16 or later) booted with `kvm-arm.mode=nested`.
func nestedVirtualizationEnabled(accel string, qemuVer *semver.Version) (bool, error) {
	if accel != "kvm" {
		return false, nil
	}
	minVer := semver.New("10.1.0")
	if qemuVer != nil && qemuVer.LessThan(*minVer) {
		return false, fmt.Errorf("QEMU %v is too old, %v or later is required for nested virtualization with KVM", qemuVer, minVer)
	}
	return true, nil
}
