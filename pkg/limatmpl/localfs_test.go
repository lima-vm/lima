// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limatmpl_test

import (
	"runtime"
	"testing"
	"testing/fstest"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatmpl"
	"github.com/lima-vm/lima/v2/pkg/plugins"
	"github.com/lima-vm/lima/v2/pkg/templatestore"
)

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func TestEmbedTemplateFromLayer(t *testing.T) {
	ctx := templatestore.WithLayers(t.Context(), []templatestore.Layer{
		{FS: fstest.MapFS{"layered.yaml": file("vmType: qemu\n")}},
	})
	tmpl := &limatmpl.Template{Bytes: []byte("base: template:layered\narch: aarch64\n"), Locator: "tmpl.yaml"}
	assert.NilError(t, tmpl.Embed(ctx, true, false))
	assert.NilError(t, tmpl.Unmarshal())
	assert.Equal(t, *tmpl.Config.VMType, "qemu")
	assert.Equal(t, *tmpl.Config.Arch, "aarch64")
}

func TestReadFromLocalFS(t *testing.T) {
	ctx := limatmpl.WithLocalFS(t.Context(), fstest.MapFS{
		"webapp.yaml":     file("base: common.yaml\nprovision:\n- file: setup.sh\n"),
		"common.yaml":     file("vmType: qemu\n"),
		"setup.sh":        file("#!/bin/sh\necho setup\n"),
		"lib/common.yaml": file("arch: aarch64\n"),
	})
	tmpl, err := limatmpl.Read(ctx, "", "/webapp.yaml")
	assert.NilError(t, err)
	assert.NilError(t, tmpl.Embed(ctx, true, false))
	assert.NilError(t, tmpl.Unmarshal())
	assert.Equal(t, *tmpl.Config.VMType, "qemu")
	assert.Equal(t, len(tmpl.Config.Provision), 1)
	assert.Equal(t, *tmpl.Config.Provision[0].Script, "#!/bin/sh\necho setup\n")

	locators := []string{"/lib/common.yaml"}
	// Read rejects a file URL whose path is not absolute. On Windows, "/lib/common.yaml" is not
	// absolute because it lacks a drive letter.
	if runtime.GOOS != "windows" {
		locators = append(locators, "file:///lib/common.yaml")
	}
	for _, locator := range locators {
		tmpl, err := limatmpl.Read(ctx, "", locator)
		assert.NilError(t, err, locator)
		assert.Equal(t, string(tmpl.Bytes), "arch: aarch64\n", locator)
	}
}

func TestLocalFSPath(t *testing.T) {
	cases := map[string]string{"/foo/bar.yaml": "foo/bar.yaml"}
	if runtime.GOOS == "windows" {
		cases[`C:\foo\bar.yaml`] = "foo/bar.yaml"
		cases[`\\server\share\x.yaml`] = "x.yaml"
	}
	for in, want := range cases {
		assert.Equal(t, limatmpl.LocalFSPath(in), want, in)
	}
}

func TestLocalFSSkipsURLPlugins(t *testing.T) {
	discover := plugins.Discover
	t.Cleanup(func() { plugins.Discover = discover })
	called := false
	plugins.Discover = func() ([]plugins.Plugin, error) {
		called = true
		return []plugins.Plugin{{Name: "url-fake", Path: "/nonexistent/limactl-url-fake"}}, nil
	}
	_, err := limatmpl.Read(limatmpl.WithLocalFS(t.Context(), fstest.MapFS{}), "", "fake:foo")
	assert.Assert(t, err != nil)
	assert.Assert(t, !called, "plugins.Discover was called")
}
