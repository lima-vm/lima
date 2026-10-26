// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package dirnames

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestLimaDirRelative(t *testing.T) {
	cwd, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)
	t.Chdir(cwd)
	t.Setenv("LIMA_HOME", "lima")

	dir, err := LimaDir()
	assert.NilError(t, err)
	assert.Equal(t, dir, filepath.Join(cwd, "lima"))

	assert.NilError(t, os.Mkdir("lima", 0o700))
	dir, err = LimaDir()
	assert.NilError(t, err)
	assert.Equal(t, dir, filepath.Join(cwd, "lima"))
}
