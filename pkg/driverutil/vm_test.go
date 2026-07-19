// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package driverutil

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/ptr"
)

func TestValidateContainerDriverConfig(t *testing.T) {
	vmType := limatype.DC
	arch := limatype.X8664
	linuxOS := limatype.LINUX
	windowsOS := limatype.WINDOWS

	tests := []struct {
		name        string
		cfg         *limatype.LimaYAML
		expectedErr string
	}{
		{
			name: "Valid DC config",
			cfg: &limatype.LimaYAML{
				VMType:    &vmType,
				Arch:      &arch,
				OS:        &linuxOS,
				MountType: ptr.Of(limatype.REVSSHFS),
				Images: []limatype.Image{
					{
						File: limatype.File{
							Location: "rootfs.tar.gz",
							Arch:     arch,
						},
					},
				},
			},
			expectedErr: "",
		},
		{
			name: "Invalid mount type for DC",
			cfg: &limatype.LimaYAML{
				VMType:    &vmType,
				Arch:      &arch,
				OS:        &linuxOS,
				MountType: ptr.Of(limatype.NINEP),
			},
			expectedErr: "field `mountType` must be `reverse-sshfs` for dc driver, got `9p`",
		},
		{
			name: "Invalid guest OS",
			cfg: &limatype.LimaYAML{
				VMType: &vmType,
				Arch:   &arch,
				OS:     &windowsOS,
			},
			expectedErr: "guest OS \"Windows\" is not supported for dc driver; only Linux guest OS is supported",
		},
		{
			name: "TPM enabled error",
			cfg: &limatype.LimaYAML{
				VMType: &vmType,
				Arch:   &arch,
				OS:     &linuxOS,
				TPM:    ptr.Of(true),
			},
			expectedErr: "field `tpm` is not supported on dc driver",
		},
		{
			name: "Non-tarball image format error",
			cfg: &limatype.LimaYAML{
				VMType: &vmType,
				Arch:   &arch,
				OS:     &linuxOS,
				Images: []limatype.Image{
					{
						File: limatype.File{
							Location: "rootfs.qcow2",
							Arch:     arch,
						},
					},
				},
			},
			expectedErr: "unsupported image type for dc: \"rootfs.qcow2\". dc only supports importing tar archive root filesystems",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerDriverConfig(tt.cfg, "dc", []limatype.MountType{limatype.REVSSHFS})
			if tt.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.expectedErr)
			}
		})
	}
}
