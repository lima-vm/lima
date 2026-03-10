// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl_test

import (
	"fmt"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatmpl"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestInstNameFromImageURL(t *testing.T) {
	t.Run("strips image format and compression method", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("linux.iso.bz2", "unknown")
		assert.Equal(t, name, "linux")
	})
	t.Run("removes generic tags", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("linux-linux_cloudimg.base-x86_64.raw", "unknown")
		assert.Equal(t, name, "linux-x86_64")
	})
	t.Run("removes Alpine `nocloud_` prefix", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("nocloud_linux-x86_64.raw", "unknown")
		assert.Equal(t, name, "linux-x86_64")
	})
	t.Run("removes date tag", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("linux-20250101.raw", "unknown")
		assert.Equal(t, name, "linux")
	})
	t.Run("removes date tag including time", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("linux-20250101-2000.raw", "unknown")
		assert.Equal(t, name, "linux")
	})
	t.Run("removes date tag including zero time", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("linux-20250101.0.raw", "unknown")
		assert.Equal(t, name, "linux")
	})
	t.Run("replace arch with archlinux", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("arch-aarch64.raw", "unknown")
		assert.Equal(t, name, "archlinux-aarch64")
	})
	t.Run("don't replace arch in the middle of the name", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("my-arch-aarch64.raw", "unknown")
		assert.Equal(t, name, "my-arch-aarch64")
	})
	t.Run("removes native arch", func(t *testing.T) {
		arch := limatype.NewArch(runtime.GOARCH)
		image := fmt.Sprintf("linux_cloudimg.base-%s.qcow2.gz", arch)
		name := limatmpl.InstNameFromImageURL(image, arch)
		assert.Equal(t, name, "linux")
	})
	t.Run("removes redundant major version", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("rocky-8-8.10.raw", "unknown")
		assert.Equal(t, name, "rocky-8.10")
	})
	t.Run("don't remove non-redundant major version", func(t *testing.T) {
		name := limatmpl.InstNameFromImageURL("rocky-8-9.10.raw", "unknown")
		assert.Equal(t, name, "rocky-8-9.10")
	})
}

func TestReadImageURLRespectsName(t *testing.T) {
	imageURL := "https://download.freebsd.org/releases/VM-IMAGES/15.0-RELEASE/aarch64/Latest/FreeBSD-15.0-RELEASE-arm64-aarch64-BASIC-CLOUDINIT-zfs.raw.xz"
	t.Run("--name flag overrides image-derived name", func(t *testing.T) {
		tmpl, err := limatmpl.Read(t.Context(), "freebsd", imageURL)
		assert.NilError(t, err)
		assert.Equal(t, tmpl.Name, "freebsd")
	})
	t.Run("name is derived from image URL when --name is empty", func(t *testing.T) {
		tmpl, err := limatmpl.Read(t.Context(), "", imageURL)
		assert.NilError(t, err)
		assert.Assert(t, tmpl.Name != "")
		assert.Assert(t, tmpl.Name != "freebsd")
	})
}

func TestSeemsTemplateURL(t *testing.T) {
	arg := "template:foo/bar"
	t.Run(arg, func(t *testing.T) {
		is, name := limatmpl.SeemsTemplateURL(arg)
		assert.Equal(t, is, true)
		assert.Equal(t, name, "foo/bar")
	})
	notTemplateURLs := []string{
		"file:///foo",
		"http://foo",
		"https://foo",
		"foo",
	}
	for _, arg := range notTemplateURLs {
		t.Run(arg, func(t *testing.T) {
			is, _ := limatmpl.SeemsTemplateURL(arg)
			assert.Equal(t, is, false)
		})
	}
}

func TestSeemsHTTPURL(t *testing.T) {
	httpURLs := []string{
		"http://foo/",
		"https://foo/",
	}
	for _, arg := range httpURLs {
		t.Run(arg, func(t *testing.T) {
			assert.Equal(t, limatmpl.SeemsHTTPURL(arg), true)
		})
	}
	notHTTPURLs := []string{
		"file:///foo",
		"template:foo",
		"foo",
	}
	for _, arg := range notHTTPURLs {
		t.Run(arg, func(t *testing.T) {
			assert.Equal(t, limatmpl.SeemsHTTPURL(arg), false)
		})
	}
}

func TestSeemsFileURL(t *testing.T) {
	arg := "file:///foo"
	t.Run(arg, func(t *testing.T) {
		assert.Equal(t, limatmpl.SeemsFileURL(arg), true)
	})
	notFileURLs := []string{
		"http://foo",
		"https://foo",
		"template:foo",
		"foo",
	}
	for _, arg := range notFileURLs {
		t.Run(arg, func(t *testing.T) {
			assert.Equal(t, limatmpl.SeemsFileURL(arg), false)
		})
	}
}

// TestRead tests that OS and architecture are correctly inferred from the image URL.
func TestRead(t *testing.T) {
	tests := []struct {
		name         string
		locator      string
		expectedName string
		expectedOS   limatype.OS
		expectedArch limatype.Arch
	}{
		{
			locator:      "http://ftp-archive.freebsd.org/pub/FreeBSD-Archive/old-releases/VM-IMAGES/15.0-RELEASE/amd64/Latest/FreeBSD-15.0-RELEASE-amd64-BASIC-CLOUDINIT-zfs.raw.xz",
			expectedName: "freebsd-15.0-amd64-zfs",
			expectedOS:   limatype.FREEBSD,
			expectedArch: limatype.X8664,
		},
		{
			locator:      "https://download.freebsd.org/ftp/snapshots/VM-IMAGES/16.0-CURRENT/aarch64/Latest/FreeBSD-16.0-CURRENT-arm64-aarch64-BASIC-CLOUDINIT-ufs.qcow2.xz",
			expectedName: "freebsd-16.0-arm64-ufs",
			expectedOS:   limatype.FREEBSD,
			expectedArch: limatype.AARCH64,
		},
		{
			locator:      "https://updates.cdn-apple.com/2025SummerFCS/fullrestores/093-10809/CFD6DD38-DAF0-40DA-854F-31AAD1294C6F/UniversalMac_15.6.1_24G90_Restore.ipsw",
			expectedName: "macos-15.6.1",
			expectedOS:   limatype.DARWIN,
			expectedArch: limatype.AARCH64,
		},
		{
			locator:      "https://updates.cdn-apple.com/2026WinterFCS/fullrestores/047-60229/6D5DBEA5-75A0-4BEF-ACC9-5ACF9B8DF6B7/UniversalMac_26.3_25D125_Restore.ipsw",
			expectedName: "macos-26.3",
			expectedOS:   limatype.DARWIN,
			expectedArch: limatype.AARCH64,
		},
		{
			locator:      "file:///somewhere/macOS-26.ipsw",
			expectedName: "macos-26",
			expectedOS:   limatype.DARWIN,
			expectedArch: limatype.AARCH64,
		},
		{
			locator:      "file:///somewhere/my-custom-macos.img",
			expectedName: "my-custom-macos",
			expectedOS:   limatype.DARWIN,
			expectedArch: limatype.AARCH64,
		},
		{
			locator:      "file:///somewhere/something.ipsw",
			expectedName: "something",
			expectedOS:   limatype.DARWIN,
			expectedArch: limatype.AARCH64,
		},
		{
			locator:      "https://cloud-images.ubuntu.com/releases/noble/release-20260209/ubuntu-24.04-server-cloudimg-amd64.img",
			expectedName: "ubuntu-24.04-amd64",
			expectedOS:   limatype.LINUX,
			expectedArch: limatype.X8664,
		},
		{
			locator:      "https://cloud-images.ubuntu.com/releases/noble/release-20260209/ubuntu-24.04-server-cloudimg-arm64.img",
			expectedName: "ubuntu-24.04-arm64",
			expectedOS:   limatype.LINUX,
			expectedArch: limatype.AARCH64,
		},
	}
	for _, tt := range tests {
		t.Run(tt.locator, func(t *testing.T) {
			tmpl, err := limatmpl.Read(t.Context(), "", tt.locator)
			assert.NilError(t, err)
			assert.Equal(t, tmpl.Name, tt.expectedName)
			err = tmpl.Unmarshal()
			assert.NilError(t, err)
			assert.Assert(t, tmpl.Config.OS != nil, "os must be set")
			assert.Equal(t, *tmpl.Config.OS, tt.expectedOS)
			assert.Assert(t, tmpl.Config.Arch != nil, "arch must be set")
			assert.Equal(t, *tmpl.Config.Arch, tt.expectedArch)
			assert.Assert(t, len(tmpl.Config.Images) == 1, "expected exactly one image entry")
			assert.Equal(t, tmpl.Config.Images[0].File.Location, tt.locator)
			assert.Equal(t, tmpl.Config.Images[0].File.Arch, tt.expectedArch)
		})
	}
}
