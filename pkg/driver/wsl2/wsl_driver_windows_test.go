// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"context"
	"runtime"
	"testing"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"gotest.tools/v3/assert"
)

func TestValidateConfigImages(t *testing.T) {
	var arch limatype.Arch
	if runtime.GOARCH == "amd64" {
		arch = limatype.X8664
	} else if runtime.GOARCH == "arm64" {
		arch = limatype.AARCH64
	} else {
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
			expectedErr: "unsupported image type for WSL2: \"/path/to/rootfs.qcow2\". WSL2 driver requires a tarball root filesystem, not a standard VM disk image (.qcow2, .raw, etc.)",
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
			expectedErr: "unsupported image type for WSL2: \"/path/to/rootfs.qcow2.gz\". WSL2 driver requires a tarball root filesystem, not a standard VM disk image (.qcow2, .raw, etc.)",
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
			expectedErr: "unsupported image type for WSL2: \"/path/to/rootfs.raw\". WSL2 driver requires a tarball root filesystem, not a standard VM disk image (.qcow2, .raw, etc.)",
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
			expectedErr: "unsupported image type for WSL2: \"/path/to/rootfs.squashfs\". WSL2 cannot natively import SquashFS images (see https://github.com/microsoft/WSL/issues/4736); please convert the image to a tarball first",
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
			expectedErr: "unsupported image type for WSL2: \"/path/to/rootfs.zip\". A tarball root filesystem (.tar, .tar.gz, .tar.xz, etc.) is required",
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
			err := validateConfig(context.Background(), cfg)
			if tt.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}
