// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

func writePlugin(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, "limactl-"+name)
	assert.NilError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755))
	return path
}

func TestDiscoverInFirstDirectoryWins(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugins need a PATHEXT extension on Windows")
	}
	first, second := t.TempDir(), t.TempDir()
	want := writePlugin(t, first, "foo")
	writePlugin(t, second, "foo")
	bar := writePlugin(t, second, "bar")

	got := pluginsInDirs([]string{first, second})
	assert.DeepEqual(t, got, []Plugin{
		{Name: "bar", Path: bar},
		{Name: "foo", Path: want},
	})
}

func TestDiscoverBundledIgnoresPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("plugins need a PATHEXT extension on Windows")
	}
	pathDir := t.TempDir()
	shadow := writePlugin(t, pathDir, "mcp")
	t.Setenv("PATH", pathDir)

	for _, plugin := range DiscoverBundled() {
		assert.Assert(t, plugin.Path != shadow, "plugin from $PATH was returned: %s", plugin.Path)
	}
}
