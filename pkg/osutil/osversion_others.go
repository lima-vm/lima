//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"errors"

	"github.com/coreos/go-semver/semver"
)

// ProductVersion returns the OS product version, not the kernel version.
func ProductVersion() (*semver.Version, error) {
	return nil, errors.New("not implemented")
}
