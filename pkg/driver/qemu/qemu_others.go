//go:build !(linux || darwin) || !(amd64 || arm64)

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import "github.com/coreos/go-semver/semver"

// nestedVirtualizationEnabled reports whether the host can provide nested virtualization
// for the given accelerator.
//
// This stub is normally unreachable: for aarch64 guests the function is only called when
// the host is linux/arm64 or darwin/arm64, and for x86_64 guests the caller guards with
// accel == "kvm" (i.e. linux/amd64). If reached via TCG cross-compilation, nested
// virtualization is not available.
func nestedVirtualizationEnabled(_ string, _ *semver.Version) (bool, error) {
	return false, nil
}
