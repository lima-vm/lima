//go:build darwin && amd64

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import "github.com/coreos/go-semver/semver"

// nestedVirtualizationEnabled reports whether the host can provide nested virtualization
// for the given accelerator.
//
// On Intel Macs, HVF does not support nested virtualization.
// This stub is normally unreachable: for x86_64 guests the caller guards with
// accel == "kvm", and for aarch64 guests the accelerator is TCG (not native).
func nestedVirtualizationEnabled(_ string, _ *semver.Version) (bool, error) {
	return false, nil
}
