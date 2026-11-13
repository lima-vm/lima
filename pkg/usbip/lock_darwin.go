// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
)

var revokeWait = 10 * time.Second

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
// ponytail: per LIMA_HOME, two LIMA_HOMEs can still race for one device.
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

func checkGrant(instDir string, info DeviceInfo) error {
	allow, err := ReadAllowlist(instDir)
	if err != nil {
		return err
	}
	if !Allowed(allow, EntryFor(info)) {
		return fmt.Errorf("usb device %s not permitted by allowlist", info.Busid)
	}
	return nil
}

// Detach drops the grant, then reads the holder. This does the reverse, so one side sees the other.
func lockGranted(instDir, holder string, info DeviceInfo) (*os.File, error) {
	lock, err := lockBusid(info.Busid, holder)
	if err != nil {
		return nil, err
	}
	if err := checkGrant(instDir, info); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return lock, nil
}

// RealName is the on-disk spelling of an instance dir: APFS opens vm1 as VM1 too, and holders compare by name.
func RealName(dir string) string {
	base, parent := filepath.Base(dir), filepath.Dir(dir)
	want, err := os.Stat(dir)
	if err != nil {
		return base
	}
	entries, _ := os.ReadDir(parent)
	for _, e := range entries {
		if !strings.EqualFold(e.Name(), base) {
			continue
		}
		if fi, err := os.Stat(filepath.Join(parent, e.Name())); err == nil && os.SameFile(fi, want) {
			return e.Name()
		}
	}
	return base
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

// Revoke ends the session through its Gone poll and waits for the release.
func Revoke(busid, holder string) error {
	if name, held := Holder(busid); !held || name != holder {
		return nil
	}
	path, err := lockPath(busid)
	if err != nil {
		return err
	}
	if err := os.Truncate(path, 0); err != nil {
		return err
	}
	for deadline := time.Now().Add(revokeWait); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if _, held := Holder(busid); !held {
			return nil
		}
	}
	return fmt.Errorf("device %s still held after %s", busid, revokeWait)
}

type lockedDevice struct {
	Device
	lock        *os.File
	holder      string
	reenumerate func()
}

// limactl empties the lock file on detach, so the session ends as if unplugged.
func (d *lockedDevice) Gone() bool {
	return readHolder(d.lock) != d.holder || d.Device.Gone()
}

func (d *lockedDevice) Close() error {
	defer d.lock.Close()
	// Only on detach: a reboot takeover needs this registry entry, because the grant names it.
	if readHolder(d.lock) != d.holder {
		d.reenumerate()
	}
	return d.Device.Close()
}
