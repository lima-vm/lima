// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usrlocal

import (
	"io/fs"
	"testing"

	"gotest.tools/v3/assert"
)

func TestReadFile(t *testing.T) {
	// Test reading containerd.yaml or networks.TEMPLATE.yaml from share/lima/defaults
	b, err := ReadFile("defaults/containerd.yaml")
	if err != nil {
		t.Logf("ReadFile(defaults/containerd.yaml) failed: %v", err)
	} else {
		assert.Assert(t, len(b) > 0)
	}
}

func TestDirFS(t *testing.T) {
	// Test DirFS for cidata.TEMPLATE.d
	f, err := DirFS("cidata.TEMPLATE.d")
	if err != nil {
		t.Logf("DirFS(cidata.TEMPLATE.d) failed: %v", err)
	} else {
		entries, err := fs.ReadDir(f, ".")
		assert.NilError(t, err)
		assert.Assert(t, len(entries) > 0)
	}
}
