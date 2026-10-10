//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import "errors"

// Sudoers is only implemented on macOS, where the VZ driver needs a privileged
// helper to open host block devices.
func Sudoers(_ []string) (string, error) {
	return "", errors.New("host block devices are only supported on macOS")
}
