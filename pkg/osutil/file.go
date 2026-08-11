// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// RemoveStaleSocket removes path so a fresh listener can bind to it, but only
// when path is a Unix domain socket. A hostSocket comes from the instance
// config (which may originate from an untrusted template) and is validated only
// as an absolute path, so it can point at an existing directory or regular
// file. os.RemoveAll on such a path would delete it recursively; here anything
// that is not a socket is left in place and an error is returned instead. A
// non-existent path is not an error.
func RemoveStaleSocket(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove %#q: not a socket (%s)", path, fi.Mode().Type())
	}
	return os.Remove(path)
}

// WriteFileBeneathDir writes data to name without following a symlink at the
// final path component. It is meant for destinations that untrusted code may be
// able to write to, e.g. a path inside a writable mount: a symlink planted at
// name (say pointing at ~/.ssh/authorized_keys) would otherwise get the bytes
// written through the link to a file outside the intended directory.
// os.OpenRoot confines the write to name's parent directory, so a symlink whose
// target escapes that directory is refused.
func WriteFileBeneathDir(name string, data []byte, perm os.FileMode) error {
	root, err := os.OpenRoot(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.OpenFile(filepath.Base(name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
