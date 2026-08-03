//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package envutil

// PlatformCommonToolDirs contains the common tool installation directories required on the host platform.
var PlatformCommonToolDirs = []string{}

// AppendDirsToPath appends specified directories to pathEnv
// Only existing directories that are not already in PATH are added.
func AppendDirsToPath(_ string, _ []string) string {
	return ""
}
