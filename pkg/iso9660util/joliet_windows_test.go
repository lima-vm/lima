// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package iso9660util

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestWriteJolietWindows(t *testing.T) {
	if _, err := exec.LookPath("cygpath"); err != nil {
		t.Skip("cygpath is not available")
	}
	if _, err := exec.LookPath("xorrisofs"); err != nil {
		t.Skip("xorrisofs is not available")
	}

	isoPath := filepath.Join(t.TempDir(), "cidata.iso")
	err := Write(isoPath, "autounattend", []Entry{
		{Path: "autounattend.xml", Reader: strings.NewReader("<unattend/>")},
	}, WithJoliet())
	assert.NilError(t, err)

	ok, err := IsISO9660(isoPath)
	assert.NilError(t, err)
	assert.Assert(t, ok, "generated file is not a readable ISO9660 image")

	label, err := Label(isoPath)
	assert.NilError(t, err)
	assert.Equal(t, strings.TrimSpace(label), "autounattend")
}
