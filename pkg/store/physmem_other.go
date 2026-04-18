// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin

package store

// getInstancePhysicalMemory is a no-op on non-macOS platforms since the
// VZ framework and `footprint` command are macOS-specific.
func getInstancePhysicalMemory(_ string) int64 {
	return 0
}
