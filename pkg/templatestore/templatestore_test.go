// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package templatestore_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/templatestore"
)

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func TestRead(t *testing.T) {
	ctx := templatestore.WithLayers(t.Context(), []templatestore.Layer{
		{FS: fstest.MapFS{"foo.yaml": file("first")}},
		{FS: fstest.MapFS{
			"foo.yaml":          file("second"),
			"ubuntu-24.04.yaml": file("ubuntu"),
			"docker.sh":         file("script"),
		}},
	})
	for _, tc := range []struct{ name, want, err string }{
		{name: "foo", want: "first"},
		{name: "/foo", want: "first"},
		{name: "ubuntu-24.04", want: "ubuntu"},
		{name: "docker.sh", want: "script"},
		{name: "../foo", err: "must not contain"},
		{name: "bar", err: "template `bar.yaml` not found"},
	} {
		b, err := templatestore.Read(ctx, tc.name)
		if tc.err != "" {
			assert.ErrorContains(t, err, tc.err, tc.name)
			continue
		}
		assert.NilError(t, err, tc.name)
		assert.Equal(t, string(b), tc.want, tc.name)
	}
}

func TestTemplates(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(dir, "_images"), 0o755))
	for _, name := range []string{"foo.yaml", "_images/bar.yaml", ".hidden.yaml", "docker.sh"} {
		assert.NilError(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), nil, 0o644))
	}
	missing := filepath.Join(dir, "missing")
	ctx := templatestore.WithLayers(t.Context(), []templatestore.Layer{
		{FS: os.DirFS(missing), Dir: missing},
		{FS: os.DirFS(dir), Dir: dir},
		{FS: fstest.MapFS{"foo.yaml": file(""), "nested/baz.yaml": file("")}},
	})
	templates, err := templatestore.Templates(ctx)
	assert.NilError(t, err)
	assert.DeepEqual(t, templates, []templatestore.Template{
		{Name: "_images/bar", Location: filepath.Join(dir, "_images", "bar.yaml")},
		{Name: "foo", Location: filepath.Join(dir, "foo.yaml")},
		{Name: "nested/baz", Location: "template:nested/baz"},
	})
}
