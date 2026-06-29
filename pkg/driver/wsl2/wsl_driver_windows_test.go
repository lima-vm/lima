// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestValidateConfigImages(t *testing.T) {
	var arch limatype.Arch
	switch runtime.GOARCH {
	case "amd64":
		arch = limatype.X8664
	case "arm64":
		arch = limatype.AARCH64
	default:
		arch = runtime.GOARCH
	}
	vmType := limatype.WSL2

	tests := []struct {
		name        string
		images      []limatype.Image
		expectedErr string
	}{
		{
			name: "Valid tarball format .tar",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.tar",
						Arch:     arch,
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "Valid tarball format .tar.gz",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.tar.gz",
						Arch:     arch,
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "Valid tarball format .tgz",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.tgz",
						Arch:     arch,
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "Valid tarball format .tar.xz",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.tar.xz",
						Arch:     arch,
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "Unsupported VM image format .qcow2",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.qcow2",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.qcow2\". wsl2 only supports importing tar archive root filesystems, not standard VM disk images",
		},
		{
			name: "Unsupported VM image format .qcow2.gz",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.qcow2.gz",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.qcow2.gz\". wsl2 only supports importing tar archive root filesystems, not standard VM disk images",
		},
		{
			name: "Unsupported VM image format .raw",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.raw",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.raw\". wsl2 only supports importing tar archive root filesystems, not standard VM disk images",
		},
		{
			name: "Unsupported SquashFS image format",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.squashfs",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.squashfs\". wsl2 does not natively support importing SquashFS images; please convert the image to a tar archive before importing",
		},
		{
			name: "Generic unsupported format",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.zip",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.zip\". A tar archive root filesystem (.tar, .tar.gz, .tar.xz, etc.) is required",
		},
		{
			name: "Unsupported VM image format .qcow2.zst",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.qcow2.zst",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.qcow2.zst\". wsl2 only supports importing tar archive root filesystems, not standard VM disk images",
		},
		{
			name: "Unsupported VM image format .raw.bz2",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.raw.bz2",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.raw.bz2\". wsl2 only supports importing tar archive root filesystems, not standard VM disk images",
		},
		{
			name: "Unsupported SquashFS image format .squashfs.gz",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.squashfs.gz",
						Arch:     arch,
					},
				},
			},
			expectedErr: "unsupported image type for wsl2: \"/path/to/rootfs.squashfs.gz\". wsl2 does not natively support importing SquashFS images; please convert the image to a tar archive before importing",
		},
		{
			name: "Different arch is ignored",
			images: []limatype.Image{
				{
					File: limatype.File{
						Location: "/path/to/rootfs.qcow2",
						Arch:     "different_arch",
					},
				},
			},
			expectedErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &limatype.LimaYAML{
				VMType: &vmType,
				Arch:   &arch,
				Images: tt.images,
			}
			err := validateConfig(t.Context(), cfg)
			if tt.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}
