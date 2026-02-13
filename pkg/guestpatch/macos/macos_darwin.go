// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package macos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/imgutil/nativeimgutil/asifutil"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/lockutil/mntlockutil"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

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

func Patch(ctx context.Context, disk string) error {
	// Retry `diskutil image attach -plist -nomount` a few times because
	// the command occasionally returns "Resource temporarily unavailable".
	attached, err := attachImageWithRetry(ctx, disk, 3)
	if err != nil {
		return fmt.Errorf("failed to attach disk: %w", err)
	}
	if attached == nil || attached.Data == "" {
		return errors.New("failed to find data slice in attached disk")
	}
	dataDevPath := "/dev/" + attached.Data
	defer func() {
		// Just detaching the data slice is enough to let the system detach the whole ASIF.
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

	if err = patchDiskRegularPhase(ctx, dataDevPath, mnt); err != nil {
		return fmt.Errorf("failed to patch macOS disk (phase 1/2): %w", err)
	}

	// Fix up the file ownership inside the disk.
	// Invokes sudo.
	if err = patchDiskPrivilegedPhase(ctx, dataDevPath, mnt); err != nil {
		return fmt.Errorf("failed to patch macOS disk (phase 2/2): %w", err)
	}

	return nil
}

func patchDiskRegularPhase(ctx context.Context, dataSliceDevice, mnt string) error {
	// Enable "noowners" to allow non-root users to write to the mounted volume.
	// The ownership is fixed up in patchDiskPrivilegedPhase.
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

func patchDiskPrivilegedPhase(ctx context.Context, dataSliceDevice, mnt string) error {
	if err := osutil.Mount(ctx, "apfs", dataSliceDevice, mnt, nil); err != nil {
		return err
	}
	defer func() {
		if err := osutil.Umount(ctx, mnt); err != nil {
			logrus.WithError(err).Warnf("failed to unmount %q", mnt)
		}
	}()
	chownCmd := chownCommand(mnt)
	cmd := exec.CommandContext(ctx, "sudo", chownCmd...)
	logrus.Infof("Executing command (chowning the newly installed files to root:wheel, the host password may be required): \n%v", cmd.Args)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to execute command %v: %w (output=%q)", cmd.Args, err, output)
	}
	return nil
}

func chownCommand(mnt string) []string {
	return []string{
		"/usr/sbin/chown",
		"root:wheel",
		filepath.Join(mnt, "private/var/db/.AppleSetupDone"),
		filepath.Join(mnt, "Library/User Template/.skipbuddy"),
		filepath.Join(mnt, "usr/local/sbin"),
		filepath.Join(mnt, "usr/local/sbin/lima-macos-init.sh"),
		filepath.Join(mnt, "Library/LaunchDaemons/io.lima-vm.lima-macos-init.plist"),
	}
}

// PrivilegedCommands returns a list of possible privileged commands to be executed on the host to patch the macOS disk.
// To be used by `limactl sudoers`.
func PrivilegedCommands() ([][]string, error) {
	limaMntDir, err := dirnames.LimaMntDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get Lima mount directory: %w", err)
	}
	var res [][]string
	slotIDs := mntlockutil.PossibleSlotIDs()
	for _, slotID := range slotIDs {
		mnt := filepath.Join(limaMntDir, slotID)
		res = append(res, chownCommand(mnt))
	}
	return res, nil
}
