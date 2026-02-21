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

package mntlockutil

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/lima-vm/lima/v2/pkg/lockutil"
)

// PossibleSlotIDs returns the possible slot IDs for mount points.
func PossibleSlotIDs() []string {
	return []string{"0"}
}

// AcquireSlot acquires a mount slot (currently always "0") under limaMntDir.
// Returns filepath.Join(limaMntDir, slotID) and a release function that releases the slot.
func AcquireSlot(limaMntDir string) (dir string, release func() error, err error) {
	slotIDs := PossibleSlotIDs()
	slotID := slotIDs[0]
	dir = filepath.Join(limaMntDir, slotID)
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return "", nil, err
	}
	if err = lockutil.Flock(dirFile, unix.LOCK_EX); err != nil {
		_ = dirFile.Close()
		return "", nil, fmt.Errorf("failed to acquire lock for %q: %w", dir, err)
	}
	release = func() error {
		if unlockErr := lockutil.Flock(dirFile, unix.LOCK_UN); unlockErr != nil {
			return fmt.Errorf("failed to release lock for %q: %w", dir, unlockErr)
		}
		return dirFile.Close()
	}
	return dir, release, nil
}
