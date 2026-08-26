// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

// nestedVirtualizationEnabled reports whether the host can provide nested virtualization
// for the given guest configuration.
//
// On aarch64 macOS, HVF nested virtualization requires QEMU 11.1.0 or later.
// https://wiki.qemu.org/ChangeLog/11.1
func nestedVirtualizationEnabled(arch, accel, _ string, qemuVer *semver.Version) (bool, error) {
	if arch != limatype.AARCH64 {
		logrus.Warnf("field `nestedVirtualization` is not supported for architecture %#q, ignoring", arch)
		return false, nil
	}
	if accel != "hvf" {
		logrus.Warnf("nested virtualization is not supported with accelerator %s on this platform, ignoring", strings.ToUpper(accel))
		return false, nil
	}
	minVer := semver.New("11.1.0")
	if qemuVer != nil && qemuVer.LessThan(*minVer) {
		return false, fmt.Errorf("QEMU %v is too old, %v or later is required for nested virtualization with HVF", qemuVer, minVer)
	}
	return true, nil
}
