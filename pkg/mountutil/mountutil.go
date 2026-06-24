// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package mountutil provides helpers for computing guest mount parameters
// (filesystem type, mount options, mount tag) that are shared between the
// boot-time cloud-init configuration and the runtime hot-mount code path.
package mountutil

import (
	"fmt"

	"github.com/docker/go-units"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

// FSType returns the guest filesystem type string for a mount type and guest OS.
// It returns an empty string for unknown mount types.
func FSType(mountType limatype.MountType, os limatype.OS) string {
	switch mountType {
	case limatype.REVSSHFS:
		return "sshfs"
	case limatype.NINEP:
		if os == limatype.FREEBSD {
			return "p9fs"
		}
		return "9p"
	case limatype.VIRTIOFS:
		return "virtiofs"
	}
	return ""
}

// Tag returns the stable mount tag for a mount (used as the 9p/virtiofs device tag).
func Tag(m *limatype.Mount) string {
	return limayaml.MountTag(m.Location, *m.MountPoint)
}

// MountOptions returns the guest mount option string for a mount given its mount
// type and the guest OS. The logic mirrors the boot-time fstab generation so that
// a folder mounted at runtime behaves identically to one mounted at boot.
func MountOptions(m *limatype.Mount, mountType limatype.MountType, os limatype.OS) (string, error) {
	fstype := FSType(mountType, os)
	options := "rw"
	if os == limatype.LINUX {
		options = "defaults"
	}
	switch fstype {
	case "9p", "p9fs", "virtiofs":
		options = "ro"
		if m.Writable != nil && *m.Writable {
			options = "rw"
		}
		if fstype == "9p" {
			if m.NineP.ProtocolVersion == nil || m.NineP.Msize == nil || m.NineP.Cache == nil {
				return "", fmt.Errorf("9p options are not set for %#q", m.Location)
			}
			options += ",trans=virtio"
			options += fmt.Sprintf(",version=%s", *m.NineP.ProtocolVersion)
			msize, err := units.RAMInBytes(*m.NineP.Msize)
			if err != nil {
				return "", fmt.Errorf("failed to parse msize for %#q: %w", m.Location, err)
			}
			options += fmt.Sprintf(",msize=%d", msize)
			options += fmt.Sprintf(",cache=%s", *m.NineP.Cache)
		}
		// don't fail the boot, if virtfs is not available
		switch os {
		case limatype.LINUX:
			options += ",nofail"
		case limatype.FREEBSD:
			options += ",failok"
		}
	}
	return options, nil
}
