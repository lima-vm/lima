// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package cacheutil

import (
	"testing"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"gotest.tools/v3/assert"
)

func boolPtr(b bool) *bool { return &b }

func TestNerdctlArchive_ReturnsBasename(t *testing.T) {
	arch := limatype.X8664
	y := &limatype.LimaYAML{
		Arch: &arch,
		Containerd: limatype.Containerd{
			System: boolPtr(true),
			User:   boolPtr(false),
			Archives: []limatype.File{
				{Location: "https://example.com/nerdctl-full-1.0.0-linux-amd64.tar.gz", Arch: limatype.X8664},
			},
		},
	}
	got := NerdctlArchive(y)
	assert.Equal(t, got, "nerdctl-full-1.0.0-linux-amd64.tar.gz")
}

func TestNerdctlArchive_ReturnsEmptyWhenContainerdDisabled(t *testing.T) {
	arch := limatype.X8664
	y := &limatype.LimaYAML{
		Arch: &arch,
		Containerd: limatype.Containerd{
			System:   boolPtr(false),
			User:     boolPtr(false),
			Archives: []limatype.File{
				{Location: "https://example.com/nerdctl-full-1.0.0-linux-amd64.tar.gz", Arch: limatype.X8664},
			},
		},
	}
	got := NerdctlArchive(y)
	assert.Equal(t, got, "")
}

func TestNerdctlArchive_ReturnsEmptyWhenArchMismatch(t *testing.T) {
	arch := limatype.X8664
	y := &limatype.LimaYAML{
		Arch: &arch,
		Containerd: limatype.Containerd{
			System: boolPtr(true),
			User:   boolPtr(false),
			Archives: []limatype.File{
				{Location: "https://example.com/nerdctl-full-1.0.0-linux-arm64.tar.gz", Arch: limatype.AARCH64},
			},
		},
	}
	got := NerdctlArchive(y)
	assert.Equal(t, got, "")
}
