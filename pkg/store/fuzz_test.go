// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/pkg/store/filenames"
)

func FuzzLoadYAMLByFilePath(f *testing.F) {
	f.Fuzz(func(t *testing.T, fileContents []byte) {
		localFile := filepath.Join(t.TempDir(), "yaml_file.yml")
		err := os.WriteFile(localFile, fileContents, 0o600)
		assert.NilError(t, err)
		_, _ = LoadYAMLByFilePath(localFile)
	})
}

func FuzzInspect(f *testing.F) {
	f.Fuzz(func(t *testing.T, yml, limaVersion []byte) {
		limaDir := t.TempDir()
		t.Setenv("LIMA_HOME", limaDir)
		err := os.MkdirAll(filepath.Join(limaDir, "fuzz-instance"), 0o700)
		assert.NilError(t, err)
		ymlFile := filepath.Join(limaDir, "fuzz-instance", filenames.LimaYAML)
		limaVersionFile := filepath.Join(limaDir, filenames.LimaVersion)
		err = os.WriteFile(ymlFile, yml, 0o600)
		assert.NilError(t, err)
		err = os.WriteFile(limaVersionFile, limaVersion, 0o600)
		assert.NilError(t, err)
		_, _ = Inspect("fuzz-instance")
	})
}
