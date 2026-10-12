// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGenCLIDocsy(t *testing.T) {
	dir := t.TempDir()
	assert.NilError(t, genCLIDocsy(newApp(), dir))

	for file, title := range map[string]string{
		"limactl_mcp.md":       "title: mcp\n",
		"limactl_mcp_info.md":  "title: mcp info\n",
		"limactl_mcp_serve.md": "title: mcp serve\n",
	} {
		b, err := os.ReadFile(filepath.Join(dir, file))
		assert.NilError(t, err)
		assert.Assert(t, strings.Contains(string(b), title), "%s: %q not found", file, title)
		assert.Assert(t, strings.Contains(string(b), "limactl mcp"), "%s: command path not found", file)
	}
	_, err := os.Stat(filepath.Join(dir, "limactl_mcp_generate-doc.md"))
	assert.Assert(t, os.IsNotExist(err), "hidden command should not be documented")
}
