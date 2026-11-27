// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestFactoryResetRemovesSubdirectories(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)

	instDir := filepath.Join(limaHome, "inst1")
	assert.NilError(t, os.MkdirAll(instDir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.LimaYAML), []byte("images: [{location: /}]\nplain: true\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.LimaVersion), []byte("2.3.0\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.VzIdentifier), []byte("id\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.Disk), []byte("disk"), 0o644))

	for _, subdir := range []string{filenames.SwtpmDir, filenames.SocketDir, filenames.CIDataISODir} {
		dirPath := filepath.Join(instDir, subdir)
		assert.NilError(t, os.MkdirAll(dirPath, 0o755))
		assert.NilError(t, os.WriteFile(filepath.Join(dirPath, "state"), []byte("data"), 0o644))
	}

	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())

	assert.NilError(t, factoryResetAction(cmd, []string{"inst1"}))

	for _, removed := range []string{filenames.Disk, filenames.SwtpmDir, filenames.SocketDir, filenames.CIDataISODir} {
		_, err := os.Stat(filepath.Join(instDir, removed))
		assert.Assert(t, os.IsNotExist(err), "%s should be removed by factory-reset", removed)
	}
	for _, kept := range []string{filenames.LimaYAML, filenames.LimaVersion, filenames.VzIdentifier, filenames.CloudConfig} {
		_, err := os.Stat(filepath.Join(instDir, kept))
		assert.NilError(t, err, "%s should be preserved by factory-reset", kept)
	}
}
