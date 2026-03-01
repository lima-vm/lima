// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package macos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/apfs"
	"github.com/lima-vm/lima/v2/pkg/imgutil/nativeimgutil/asifutil"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/lockutil/mntlockutil"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

// attachImageWithRetry retries `diskutil image attach` because the
// command occasionally returns "Resource temporarily unavailable".
func attachImageWithRetry(ctx context.Context, disk string, retry int) (*asifutil.AttachedDisk, error) {
	var (
		attached *asifutil.AttachedDisk
		err      error
	)
	for range retry {
		attached, err = asifutil.DiskutilImageAttachNoMount(ctx, disk)
		if !errors.Is(err, asifutil.ErrResourceTemporarilyUnavailable) {
			break
		}
		time.Sleep(3 * time.Second)
	}
	return attached, err
}

// Patch prepares a macOS guest disk for first boot. It writes the
// LaunchDaemon plist, init script, and setup markers via a noowners
// mount, then fixes file ownership by patching APFS inode records
// directly on the raw disk image. No sudo required.
func Patch(ctx context.Context, disk string) error {
	if err := patchWriteGuestFiles(ctx, disk); err != nil {
		return err
	}
	return patchFixOwnership(ctx, disk)
}

// patchWriteGuestFiles attaches the disk image, mounts the Data
// volume with noowners, writes guest files, then detaches.
func patchWriteGuestFiles(ctx context.Context, disk string) error {
	attached, err := attachImageWithRetry(ctx, disk, 3)
	if err != nil {
		return fmt.Errorf("failed to attach disk: %w", err)
	}
	if attached == nil || attached.Data == "" {
		return errors.New("failed to find data slice in attached disk")
	}
	dataDevPath := "/dev/" + attached.Data
	defer func() {
		// Detaching the data slice is enough to detach the whole ASIF.
		if err := asifutil.DetachASIF(dataDevPath); err != nil {
			logrus.WithError(err).Warnf("failed to detach %q (%q)", dataDevPath, disk)
		}
	}()

	limaMntDir, err := dirnames.LimaMntDir()
	if err != nil {
		return fmt.Errorf("failed to get Lima mount directory: %w", err)
	}

	mnt, mntRelease, err := mntlockutil.AcquireSlot(limaMntDir)
	if err != nil {
		return fmt.Errorf("failed to acquire mount slot: %w", err)
	}
	defer func() {
		if mntReleaseErr := mntRelease(); mntReleaseErr != nil {
			logrus.WithError(mntReleaseErr).Warnf("failed to release mount slot %q", mnt)
		}
	}()

	return writeGuestFiles(ctx, dataDevPath, mnt)
}

// patchFixOwnership attaches the disk image and patches APFS inode
// records on the raw container device to set root:wheel ownership.
func patchFixOwnership(ctx context.Context, disk string) error {
	attached, err := attachImageWithRetry(ctx, disk, 3)
	if err != nil {
		return fmt.Errorf("failed to attach disk for ownership fix: %w", err)
	}
	if attached == nil || attached.Data == "" {
		return errors.New("failed to find data slice in attached disk")
	}
	dataDevPath := "/dev/" + attached.Data
	defer func() {
		if err := asifutil.DetachASIF(dataDevPath); err != nil {
			logrus.WithError(err).Warnf("failed to detach %q (%q)", dataDevPath, disk)
		}
	}()

	if attached.Container == "" {
		return errors.New("diskutil did not report an APFS container device")
	}

	// Patch APFS inode records via the raw container device.
	// The noowners mount stores files with uid=99 (nobody);
	// LaunchDaemon plists must be owned by root:wheel for launchd
	// to load them, so we patch them to UID 0 / GID 0 directly.
	containerDev := "/dev/r" + attached.Container
	if err = apfs.Chown(containerDev, apfs.VolRoleData, 0, 0,
		"private/var/db/.AppleSetupDone",
		"Library/User Template/.skipbuddy",
		"usr/local/sbin",
		"usr/local/sbin/lima-macos-init.sh",
		"Library/LaunchDaemons/io.lima-vm.lima-macos-init.plist",
	); err != nil {
		return fmt.Errorf("failed to fix file ownership on disk: %w", err)
	}

	return nil
}

// writeGuestFiles mounts the data volume with noowners and writes the
// LaunchDaemon plist, init script, and setup markers.
func writeGuestFiles(ctx context.Context, dataSliceDevice, mnt string) error {
	// Mount with "noowners" so non-root users can write to the volume.
	if err := osutil.Mount(ctx, "apfs", dataSliceDevice, mnt, []string{"noowners"}); err != nil {
		return err
	}
	defer func() {
		if err := osutil.Umount(ctx, mnt); err != nil {
			logrus.WithError(err).Warnf("failed to unmount %q", mnt)
		}
	}()

	filesToTouch := []string{
		filepath.Join(mnt, "private/var/db/.AppleSetupDone"),
		filepath.Join(mnt, "Library/User Template/.skipbuddy"),
	}
	for _, file := range filesToTouch {
		if err := osutil.Touch(file); err != nil {
			return fmt.Errorf("failed to touch %q: %w", file, err)
		}
	}

	const initSh = `#!/bin/sh
set -eux
date
if [ ! -e /Volumes/cidata ]; then
  echo "/Volumes/cidata is not mounted" >&2
  exit 1
fi
exec /Volumes/cidata/lima-guestagent fake-cloud-init
`
	if err := os.MkdirAll(filepath.Join(mnt, "usr/local/sbin"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(mnt, "usr/local/sbin/lima-macos-init.sh"), []byte(initSh), 0o755); err != nil {
		return err
	}

	const plist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>io.lima-vm.lima-macos-init</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/sbin/lima-macos-init.sh</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/dev/tty.virtio</string>
    <key>StandardErrorPath</key>
    <string>/dev/tty.virtio</string>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(mnt, "Library/LaunchDaemons/io.lima-vm.lima-macos-init.plist"), []byte(plist), 0o755); err != nil {
		return err
	}
	return nil
}
