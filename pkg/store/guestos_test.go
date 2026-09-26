// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestReadGuestOSVersion(t *testing.T) {
	instDir := t.TempDir()

	version, buildVersion := ReadGuestOSVersion(instDir)
	assert.Equal(t, version, "")
	assert.Equal(t, buildVersion, "")

	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.VzGuestOSVersion), []byte("27.0.0\n"), 0o644))
	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.VzGuestOSBuildVersion), []byte("26A428\n"), 0o644))
	version, buildVersion = ReadGuestOSVersion(instDir)
	assert.Equal(t, version, "27.0.0")
	assert.Equal(t, buildVersion, "26A428")
}
