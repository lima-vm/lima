// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/lima-vm/lima/pkg/limatmpl"
	"github.com/lima-vm/lima/pkg/limayaml"
	"gotest.tools/v3/assert"
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
		arch := limayaml.NewArch(runtime.GOARCH)
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

func TestSeemsTemplateURL(t *testing.T) {
	arg := "template://foo/bar"
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
		"template://foo",
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
		"template://foo",
		"foo",
	}
	for _, arg := range notFileURLs {
		t.Run(arg, func(t *testing.T) {
			assert.Equal(t, limatmpl.SeemsFileURL(arg), false)
		})
	}
}
