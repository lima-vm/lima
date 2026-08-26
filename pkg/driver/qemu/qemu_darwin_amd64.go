// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
)

// nestedVirtualizationEnabled reports whether the host can provide nested virtualization
// for the given guest configuration.
//
// On Intel Macs, HVF does not support nested virtualization.
func nestedVirtualizationEnabled(_, _, _ string, _ *semver.Version) (bool, error) {
	logrus.Warn("field `nestedVirtualization` is not supported on this platform, ignoring")
	return false, nil
}
