// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package localpathutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsTildePath returns true if the path is "~" or starts with "~/".
// This means Expand() can expand it with the home directory.
func IsTildePath(path string) bool {
	return path == "~" || strings.HasPrefix(path, "~/")
}

// Expand expands a path like "~", "~/", "~/foo".
// Paths like "~foo/bar" are unsupported.
//
// FIXME: is there an existing library for this?
func Expand(orig string) (string, error) {
	s := orig
	if s == "" {
		return "", errors.New("empty path")
	}

	if strings.HasPrefix(s, "~") {
		if IsTildePath(s) {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			s = strings.Replace(s, "~", homeDir, 1)
		} else {
			// Paths like "~foo/bar" are unsupported.
			return "", fmt.Errorf("unexpandable path %q", orig)
		}
	}
	return filepath.Abs(s)
}
