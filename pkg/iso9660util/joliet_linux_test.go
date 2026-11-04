// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package iso9660util

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestWriteJolietCommandLinux(t *testing.T) {
	binDir := t.TempDir()
	mkisofs := filepath.Join(binDir, "mkisofs")
	assert.NilError(t, os.WriteFile(mkisofs, nil, 0o755))
	t.Setenv("PATH", binDir)

	isoPath := "/tmp/lima/cidata.iso"
	workDir := "/tmp/joliet1"
	args, err := writeJolietCommand(t.Context(), isoPath, "autounattend", workDir)
	assert.NilError(t, err)
	assert.DeepEqual(t, args, []string{mkisofs, "-o", isoPath, "-J", "-V", "autounattend", workDir})
}
