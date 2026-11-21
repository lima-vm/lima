//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// From https://github.com/containerd/nerdctl/blob/v0.13.0/pkg/lockutil/lockutil_unix.go
/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package lockutil

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func WithDirLock(dir string, fn func() error) error {
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	if err := Flock(dirFile, unix.LOCK_EX); err != nil {
		return fmt.Errorf("failed to lock %#q: %w", dir, err)
	}
	defer func() {
		if err := Flock(dirFile, unix.LOCK_UN); err != nil {
			logrus.WithError(err).Errorf("failed to unlock %#q", dir)
		}
	}()
	if err := checkLockedDir(dirFile, dir); err != nil {
		return err
	}
	return fn()
}

// checkLockedDir returns ErrDirRemoved when dir no longer refers to the locked
// directory. The lock is held on the directory inode, so it does not guard a
// directory that was removed, and possibly recreated, while waiting for the lock.
func checkLockedDir(dirFile *os.File, dir string) error {
	locked, err := dirFile.Stat()
	if err != nil {
		return err
	}
	current, err := os.Stat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %#q", ErrDirRemoved, dir)
	}
	if err != nil {
		return err
	}
	if !os.SameFile(locked, current) {
		return fmt.Errorf("%w: %#q", ErrDirRemoved, dir)
	}
	return nil
}

func Flock(f *os.File, flags int) error {
	fd := int(f.Fd())
	for {
		err := unix.Flock(fd, flags)
		if err == nil || err != unix.EINTR {
			return err
		}
	}
}
