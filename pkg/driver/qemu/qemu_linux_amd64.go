// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// nestedVirtualizationEnabled reports whether the host can provide nested virtualization
// for the given guest configuration.
//
// On x86_64 Linux, nested virtualization is controlled by the host KVM module
// (the `nested` parameter of kvm_intel / kvm_amd) and is exposed to the guest via `-cpu host`.
// No minimum QEMU version is required.
func nestedVirtualizationEnabled(arch, accel, cpu string, _ *semver.Version) (bool, error) {
	if arch != limatype.X8664 {
		logrus.Warnf("field `nestedVirtualization` is not supported for architecture %#q, ignoring", arch)
		return false, nil
	}
	if accel != "kvm" {
		logrus.Warnf("field `nestedVirtualization` is not supported with accelerator %#q for architecture %#q, ignoring", accel, arch)
		return false, nil
	}
	for _, module := range []string{"kvm_intel", "kvm_amd"} {
		b, err := os.ReadFile(filepath.Join("/sys/module", module, "parameters", "nested"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			logrus.WithError(err).Warn("Failed to check whether nested virtualization is enabled in the host KVM module")
			return false, nil
		}
		enabled, err := parseKVMNestedParam(string(b))
		if err != nil {
			return false, err
		}
		if !enabled {
			return false, errors.New("nested virtualization is disabled in the host KVM module (`nested` parameter of kvm_intel / kvm_amd)")
		}
		if !strings.HasPrefix(cpu, "host") {
			logrus.Warnf("nested virtualization on x86_64 requires CPU type `host`, got %#q", cpu)
		}
		return true, nil
	}
	return false, errors.New("neither kvm_intel nor kvm_amd is loaded")
}
