// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package plugins

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestExtractDescFromScript(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		assert.NilError(t, os.WriteFile(path, []byte(content), 0o755))
		return path
	}

	script := write("script", "#!/bin/sh\n# <limactl-desc>Does things</limactl-desc>\n")
	assert.Equal(t, extractDescFromScript(script), "Does things")

	// The description is found anywhere in a script, not only near the top.
	long := write("long", "#!/bin/sh\n"+strings.Repeat("# filler\n", 10000)+"# <limactl-desc>Far down</limactl-desc>\n")
	assert.Equal(t, extractDescFromScript(long), "Far down")

	binary := write("binary", "\x7fELF <limactl-desc>Not a script</limactl-desc>\n")
	assert.Equal(t, extractDescFromScript(binary), "")

	assert.Equal(t, extractDescFromScript(write("empty", "")), "")
	assert.Equal(t, extractDescFromScript(write("short", "#")), "")
	assert.Equal(t, extractDescFromScript(filepath.Join(dir, "missing")), "")
}
