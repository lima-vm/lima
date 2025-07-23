// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"gotest.tools/v3/assert"
)

func dumpJSON(t *testing.T, d any) string {
	b, err := json.Marshal(d)
	assert.NilError(t, err)
	return string(b)
}

const emptyYAML = "{}\n"

func TestEmptyYAML(t *testing.T) {
	var y limatype.LimaYAML
	t.Log(dumpJSON(t, y))
	b, err := Marshal(&y, false)
	assert.NilError(t, err)
	assert.Equal(t, string(b), emptyYAML)
}

const defaultYAML = "{}\n"

func TestDefaultYAML(t *testing.T) {
	content, err := os.ReadFile("default.yaml")
	assert.NilError(t, err)
	// if this is the unresolved symlink as a file, then make sure to resolve it
	if runtime.GOOS == "windows" && bytes.HasPrefix(content, []byte{'.', '.'}) {
		f, err := filepath.Rel(".", string(content))
		assert.NilError(t, err)
		content, err = os.ReadFile(f)
		assert.NilError(t, err)
	}

	var y limatype.LimaYAML
	err = Unmarshal(content, &y, "")
	assert.NilError(t, err)
	y.Images = nil                // remove default images
	y.Mounts = nil                // remove default mounts
	y.Base = nil                  // remove default base templates
	y.MinimumLimaVersion = nil    // remove minimum Lima version
	y.MountTypesUnsupported = nil // remove default workaround for kernel 6.9-6.11
	t.Log(dumpJSON(t, y))
	b, err := Marshal(&y, false)
	assert.NilError(t, err)
	assert.Equal(t, string(b), defaultYAML)
}
