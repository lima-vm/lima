// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
)

func lockPath(busid string) (string, error) {
	dir, err := dirnames.LimaDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "_usbip-"+busid+".lock"), nil
}

func readHolder(f *os.File) string {
	var buf [256]byte
	n, _ := f.ReadAt(buf[:], 0)
	return string(buf[:n])
}

// The kernel drops the flock at process death.
// The lock is per LIMA_HOME, so two LIMA_HOMEs can still race for one device.
func lockBusid(busid, holder string) (*os.File, error) {
	path, err := lockPath(busid)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	// A Holder probe takes the lock for an instant, so one miss proves nothing.
	for range 5 {
		if err = lockutil.Flock(f, unix.LOCK_EX|unix.LOCK_NB); !errors.Is(err, unix.EWOULDBLOCK) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if errors.Is(err, unix.EWOULDBLOCK) {
		err = fmt.Errorf("usb device %s is held by instance %q", busid, readHolder(f))
	}
	if err == nil {
		err = f.Truncate(0)
	}
	if err == nil {
		_, err = f.WriteAt([]byte(holder), 0)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// Holder trusts the flock: a name in a free file is stale.
func Holder(busid string) (name string, held bool) {
	path, err := lockPath(busid)
	if err != nil {
		return "", false
	}
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	if lockutil.Flock(f, unix.LOCK_SH|unix.LOCK_NB) == nil {
		return "", false
	}
	return readHolder(f), true
}
