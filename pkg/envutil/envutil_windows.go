// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package envutil

import (
	"os"
	"path/filepath"
)

// PlatformCommonToolDirs contains the common tool installation directories required on the host platform.
var PlatformCommonToolDirs = []string{
	qemuDir(),
	`C:\msys64\usr\bin`,
}

// qemuDir returns the default QEMU installation directories under %PROGRAMFILES%.
func qemuDir() string {
	programfiles, ok := os.LookupEnv("PROGRAMFILES")
	if !ok {
		programfiles = `C:\Program Files`
	}
	qemu := filepath.Join(programfiles, "QEMU")
	return qemu
}

// AppendDirsToPath appends specified directories to pathEnv
// Only existing directories that are not already in PATH are added.
func AppendDirsToPath(pathEnv string, dirs []string) string {
	seen := make(map[string]struct{})
	for _, entry := range filepath.SplitList(pathEnv) {
		seen[entry] = struct{}{}
	}
	for _, dir := range dirs {
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		pathEnv += string(filepath.ListSeparator) + dir
	}
	return pathEnv
}
