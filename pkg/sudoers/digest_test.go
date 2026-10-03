// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package sudoers

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestPin(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "lima-privileged-net")
	assert.NilError(t, os.WriteFile(binary, nil, 0o755))

	const emptyFileDigest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	pinned, err := Pin(binary, binary+" start", binary+" tap")
	assert.NilError(t, err)
	assert.DeepEqual(t, pinned, []string{
		emptyFileDigest + " " + binary + " start",
		emptyFileDigest + " " + binary + " tap",
	})

	_, err = Pin(filepath.Join(t.TempDir(), "missing"), "missing start")
	assert.ErrorIs(t, err, os.ErrNotExist)
}
