//go:build !darwin && !linux

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

// BootSessionID returns an identifier of the current boot of the host.
func BootSessionID() (string, error) {
	return "", ErrBootSessionNotSupported
}
