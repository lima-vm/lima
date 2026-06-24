// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	hostagentapi "github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/mountutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

// activeMount is a runtime (hot) mount tracked by the host agent.
type activeMount struct {
	hostPath  string
	guestPath string
	mountType limatype.MountType
	writable  bool
	close     func() error
}

func (am *activeMount) apiMount() hostagentapi.Mount {
	return hostagentapi.Mount{
		ID:         am.guestPath,
		HostPath:   am.hostPath,
		MountPoint: am.guestPath,
		Type:       string(am.mountType),
		Writable:   am.writable,
	}
}

// reservedGuestMountPoints cannot be used as hot-mount points.
var reservedGuestMountPoints = []string{"/", "/bin", "/dev", "/etc", "/home", "/opt", "/sbin", "/tmp", "/usr", "/var"}

func validateHotMount(hostPath, guestPath string) error {
	if !filepath.IsAbs(guestPath) {
		return fmt.Errorf("mount point %#q must be an absolute path", guestPath)
	}
	for _, reserved := range reservedGuestMountPoints {
		if guestPath == reserved {
			return fmt.Errorf("mount point %#q is reserved", guestPath)
		}
	}
	st, err := os.Stat(hostPath)
	if err != nil {
		return fmt.Errorf("host path %#q: %w", hostPath, err)
	}
	if !st.IsDir() {
		return fmt.Errorf("host path %#q is not a directory", hostPath)
	}
	return nil
}

// buildHotMount constructs a limatype.Mount with the same defaults that FillDefault
// applies to a configured mount, so a runtime mount behaves identically to a static one.
func buildHotMount(hostPath, guestPath string, mountType limatype.MountType, writable bool) limatype.Mount {
	m := limatype.Mount{
		Location:   hostPath,
		MountPoint: ptr.Of(guestPath),
		Writable:   ptr.Of(writable),
	}
	switch mountType {
	case limatype.REVSSHFS:
		m.SSHFS = limatype.SSHFS{
			Cache:          ptr.Of(true),
			FollowSymlinks: ptr.Of(false),
			SFTPDriver:     ptr.Of(""),
		}
	case limatype.NINEP:
		cache := limayaml.Default9pCacheForRO
		if writable {
			cache = limayaml.Default9pCacheForRW
		}
		m.NineP = limatype.NineP{
			SecurityModel:   ptr.Of(limayaml.Default9pSecurityModel),
			ProtocolVersion: ptr.Of(limayaml.Default9pProtocolVersion),
			Msize:           ptr.Of(limayaml.Default9pMsize),
			Cache:           ptr.Of(cache),
		}
	}
	return m
}

// MountAdd mounts a host directory into the running VM at runtime. mountType defaults
// to virtiofs. The mount is ephemeral and is not written to lima.yaml.
func (a *HostAgent) MountAdd(ctx context.Context, hostPath, guestPath string, mountType limatype.MountType, writable bool) (*hostagentapi.Mount, error) {
	if mountType == "" {
		mountType = limatype.VIRTIOFS
	}
	if err := validateHotMount(hostPath, guestPath); err != nil {
		return nil, err
	}

	a.hotMountsMu.Lock()
	defer a.hotMountsMu.Unlock()
	if _, exists := a.hotMounts[guestPath]; exists {
		return nil, fmt.Errorf("mount point %#q is already mounted", guestPath)
	}

	m := buildHotMount(hostPath, guestPath, mountType, writable)
	var closeFn func() error
	switch mountType {
	case limatype.REVSSHFS:
		mnt, err := a.setupMount(ctx, m)
		if err != nil {
			return nil, err
		}
		closeFn = mnt.close
	case limatype.NINEP, limatype.VIRTIOFS:
		cf, err := a.hotPlugMount(ctx, m, mountType, writable)
		if err != nil {
			return nil, err
		}
		closeFn = cf
	default:
		return nil, fmt.Errorf("unsupported mount type %#q", mountType)
	}

	am := &activeMount{
		hostPath:  hostPath,
		guestPath: guestPath,
		mountType: mountType,
		writable:  writable,
		close:     closeFn,
	}
	a.hotMounts[guestPath] = am
	res := am.apiMount()
	return &res, nil
}

// hotPlugMount attaches a 9p/virtiofs device via the driver and mounts it in the guest.
func (a *HostAgent) hotPlugMount(ctx context.Context, m limatype.Mount, mountType limatype.MountType, writable bool) (func() error, error) {
	hp, ok := a.driver.(driver.FSHotPlugger)
	if !ok {
		return nil, driver.ErrFSHotPlugUnsupported
	}
	tag := mountutil.Tag(&m)
	req := &driver.HotPlugFSRequest{
		Type:     mountType,
		HostPath: m.Location,
		Tag:      tag,
		Writable: writable,
	}
	if mountType == limatype.NINEP {
		req.NineP = &m.NineP
	}
	resp, err := hp.HotPlugFS(ctx, req)
	if err != nil {
		return nil, err
	}
	deviceID := resp.DeviceID

	unplug := func() error {
		return hp.HotUnplugFS(context.Background(), &driver.HotUnplugFSRequest{DeviceID: deviceID})
	}

	fstype := mountutil.FSType(mountType, *a.instConfig.OS)
	opts, err := mountutil.MountOptions(&m, mountType, *a.instConfig.OS)
	if err != nil {
		_ = unplug()
		return nil, err
	}
	mountPoint := *m.MountPoint
	// The guest may take a moment to enumerate the freshly hot-plugged PCI device
	// before its mount tag becomes usable, so retry the mount for a short while.
	// fstype and opts are generated from a fixed, safe set; mountPoint and tag are
	// single-quote escaped because mountPoint is user-supplied.
	mountScript := fmt.Sprintf(`#!/bin/sh
set -eu
sudo mkdir -p %[1]s
i=0
while [ "$i" -lt 30 ]; do
	if sudo mount -t %[2]s -o %[3]s %[4]s %[1]s; then exit 0; fi
	i=$((i + 1))
	sleep 0.5
done
echo "timed out waiting for device %[4]s to appear" >&2
exit 1
`, shellEscape(mountPoint), fstype, opts, shellEscape(tag))
	if _, stderr, err := a.guestExec(mountScript, "hot-mount "+mountPoint); err != nil {
		_ = unplug()
		return nil, fmt.Errorf("failed to mount %#q in guest: %w (stderr=%q)", mountPoint, err, stderr)
	}

	closeFn := func() error {
		umountScript := fmt.Sprintf("#!/bin/sh\nset -eu\nsudo umount %s\n", shellEscape(mountPoint))
		if _, stderr, err := a.guestExec(umountScript, "hot-unmount "+mountPoint); err != nil {
			logrus.Warnf("failed to umount %#q in guest: %v (stderr=%q)", mountPoint, err, stderr)
		}
		return unplug()
	}
	return closeFn, nil
}

// shellEscape single-quotes s for safe interpolation into a POSIX shell script.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// MountRemove unmounts a runtime mount previously added with MountAdd.
func (a *HostAgent) MountRemove(_ context.Context, guestPath string) error {
	a.hotMountsMu.Lock()
	defer a.hotMountsMu.Unlock()
	am, ok := a.hotMounts[guestPath]
	if !ok {
		return fmt.Errorf("mount point %#q is not a hot-mount", guestPath)
	}
	if err := am.close(); err != nil {
		return err
	}
	delete(a.hotMounts, guestPath)
	return nil
}

// MountList returns the active runtime mounts.
func (a *HostAgent) MountList() []hostagentapi.Mount {
	a.hotMountsMu.Lock()
	defer a.hotMountsMu.Unlock()
	res := make([]hostagentapi.Mount, 0, len(a.hotMounts))
	for _, am := range a.hotMounts {
		res = append(res, am.apiMount())
	}
	return res
}

// hotMountPoints returns the guest mount points of all active hot-mounts.
func (a *HostAgent) hotMountPoints() []string {
	a.hotMountsMu.Lock()
	defer a.hotMountsMu.Unlock()
	res := make([]string, 0, len(a.hotMounts))
	for guestPath := range a.hotMounts {
		res = append(res, guestPath)
	}
	return res
}
