// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// IsNFS checks if the path is on NFS. If the path does not exist yet, it will walk
// up parent directories until one exists, or it hits '/' or '.'.
// Any other stat errors will cause IsNFS to fail.
func IsNFS(path string) (bool, error) {
	for len(path) > 1 {
		_, err := os.Stat(path)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		path = filepath.Dir(path)
	}

	var sf unix.Statfs_t
	if err := unix.Statfs(path, &sf); err != nil {
		return false, err
	}
	return sf.Type == unix.NFS_SUPER_MAGIC, nil
}

// Inode flags from <linux/fs.h>, which golang.org/x/sys/unix does not define.
const (
	fsComprFL  = 0x00000004 // FS_COMPR_FL
	fsNoCompFL = 0x00000400 // FS_NOCOMP_FL
	fsNoCOWFL  = 0x00800000 // FS_NOCOW_FL
)

// DisableCopyOnWrite sets the No_COW attribute (`chattr +C`) on dir when dir is on btrfs,
// like libvirt does for new storage pools. Files created in dir afterwards inherit it, so
// in-place writes do not fragment VM disk images there. Files already in dir are not affected.
// On other filesystems, and with the environment variable LIMA_BTRFS_NOCOW=false,
// DisableCopyOnWrite does nothing.
func DisableCopyOnWrite(dir string) error {
	if v := os.Getenv("LIMA_BTRFS_NOCOW"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			logrus.WithError(err).Warnf("invalid LIMA_BTRFS_NOCOW value %#q", v)
		} else if !b {
			return nil
		}
	}
	return setNoCOW(dir, true)
}

// MatchCopyOnWrite gives dst the No_COW attribute of src, or takes it away when src has none,
// if both are on btrfs. btrfs cannot reflink between files with and without the attribute, so the
// directory for a copy of an instance must follow the original, whatever LIMA_BTRFS_NOCOW says.
// When src is not on btrfs, dst gets what DisableCopyOnWrite gives a new directory.
func MatchCopyOnWrite(dst, src string) error {
	if ok, err := onBtrfs(src); err != nil {
		return err
	} else if !ok {
		// No reflink is possible from another filesystem, so dst gets the default for a new instance.
		return DisableCopyOnWrite(dst)
	}
	noCOW, err := hasNoCOW(src)
	if err != nil {
		return err
	}
	return setNoCOW(dst, noCOW)
}

// WarnIfFullCopy warns that copying src to dst will copy every block when both are on btrfs
// and only one of src and the directory of dst has the No_COW attribute: btrfs cannot reflink
// between them, and the copy functions do not report whether they made a reflink.
func WarnIfFullCopy(dst, src string) {
	dir := filepath.Dir(dst)
	for _, p := range []string{dir, src} {
		if ok, err := onBtrfs(p); !ok || err != nil {
			return
		}
	}
	dirNoCOW, err := hasNoCOW(dir)
	if err != nil {
		return
	}
	srcNoCOW, err := hasNoCOW(src)
	if err == nil && dirNoCOW != srcNoCOW {
		logrus.Warnf("Copying %#q in full, because btrfs cannot share blocks between files with and without copy-on-write", src)
	}
}

// onBtrfs reports whether path is on btrfs.
func onBtrfs(path string) (bool, error) {
	var sf unix.Statfs_t
	if err := unix.Statfs(path, &sf); err != nil {
		return false, err
	}
	// Statfs_t.Type is 32-bit on some architectures, where the btrfs magic overflows int32.
	return uint32(sf.Type) == unix.BTRFS_SUPER_MAGIC, nil
}

// hasNoCOW reports whether path is on btrfs and has the No_COW attribute.
func hasNoCOW(path string) (bool, error) {
	if ok, err := onBtrfs(path); !ok || err != nil {
		return false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	flags, err := unix.IoctlGetUint32(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	return flags&fsNoCOWFL != 0, err
}

// setNoCOW sets or clears the No_COW attribute of dir when dir is on btrfs.
func setNoCOW(dir string, noCOW bool) error {
	if ok, err := onBtrfs(dir); !ok || err != nil {
		return err
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	flags, err := unix.IoctlGetUint32(int(f.Fd()), unix.FS_IOC_GETFLAGS)
	if err != nil {
		return err
	}
	newFlags := flags &^ fsNoCOWFL
	if noCOW {
		if flags&(fsComprFL|fsNoCompFL) != 0 {
			// btrfs refuses No_COW with a compression setting (EINVAL), which the user chose for this tree.
			logrus.Debugf("Keeping copy-on-write for %#q, which has a compression setting", dir)
			return nil
		}
		newFlags |= fsNoCOWFL
	}
	if newFlags == flags {
		return nil
	}
	return unix.IoctlSetPointerInt(int(f.Fd()), unix.FS_IOC_SETFLAGS, int(newFlags))
}
