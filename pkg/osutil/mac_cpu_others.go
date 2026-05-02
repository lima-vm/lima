//go:build !darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

// IsAppleSiliconM4OrNewer returns true if the host CPU is Apple M4 or newer.
func IsAppleSiliconM4OrNewer() bool {
	return false
}
