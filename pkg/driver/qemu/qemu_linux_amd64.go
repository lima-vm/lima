//go:build linux && amd64

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/coreos/go-semver/semver"
)

// nestedVirtualizationEnabled reports whether the host can provide nested virtualization
// for the given accelerator.
//
// On x86_64 Linux, nested virtualization is controlled by the host KVM module
// (the `nested` parameter of kvm_intel / kvm_amd) and is exposed to the guest via `-cpu host`.
// No minimum QEMU version is required.
func nestedVirtualizationEnabled(accel string, _ *semver.Version) (bool, error) {
	if accel != "kvm" {
		return false, nil
	}
	for _, module := range []string{"kvm_intel", "kvm_amd"} {
		b, err := os.ReadFile(filepath.Join("/sys/module", module, "parameters", "nested"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return false, err
		}
		return parseKVMNestedParam(string(b))
	}
	return false, errors.New("neither kvm_intel nor kvm_amd is loaded")
}
