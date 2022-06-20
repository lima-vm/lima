// From https://github.com/containerd/nerdctl/blob/v0.13.0/pkg/lockutil/lockutil_windows.go
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
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/sirupsen/logrus"
)

// LockFile modified from https://github.com/boltdb/bolt/blob/v1.3.1/bolt_windows.go using MIT
var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

func WithDirLock(dir string, fn func() error) error {
	dirFile, err := os.OpenFile(dir+".lock", os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	// see https://msdn.microsoft.com/en-us/library/windows/desktop/aa365203(v=vs.85).aspx
	// 1 lock immediately
	if err := lockFileEx(syscall.Handle(dirFile.Fd()), 1, 0, 1, 0, &syscall.Overlapped{}); err != nil {
		return fmt.Errorf("failed to lock %q: %w", dir, err)
	}

	defer func() {
		if err := unlockFileEx(syscall.Handle(dirFile.Fd()), 0, 1, 0, &syscall.Overlapped{}); err != nil {
			logrus.WithError(err).Errorf("failed to unlock %q", dir)
		}
	}()
	return fn()
}

func lockFileEx(h syscall.Handle, flags, reserved, locklow, lockhigh uint32, ol *syscall.Overlapped) (err error) {
	r, _, err := procLockFileEx.Call(uintptr(h), uintptr(flags), uintptr(reserved), uintptr(locklow), uintptr(lockhigh), uintptr(unsafe.Pointer(ol)))
	if r == 0 {
		return err
	}
	return nil
}

func unlockFileEx(h syscall.Handle, reserved, locklow, lockhigh uint32, ol *syscall.Overlapped) (err error) {
	r, _, err := procUnlockFileEx.Call(uintptr(h), uintptr(reserved), uintptr(locklow), uintptr(lockhigh), uintptr(unsafe.Pointer(ol)), 0)
	if r == 0 {
		return err
	}
	return nil
}
