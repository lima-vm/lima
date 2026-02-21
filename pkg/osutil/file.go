// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"errors"
	"os"
)

// FileExists reports whether path exists and is accessible.
// It returns true for any non-ErrNotExist stat result, including permission errors.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

// Touch touches a file.
func Touch(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
