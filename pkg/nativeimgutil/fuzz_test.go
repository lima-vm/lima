// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package nativeimgutil

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func FuzzConvertToRaw(f *testing.F) {
	f.Fuzz(func(t *testing.T, imgData []byte, withBacking bool, size int64) {
		srcPath := filepath.Join(t.TempDir(), "src.img")
		destPath := filepath.Join(t.TempDir(), "dest.img")
		err := os.WriteFile(srcPath, imgData, 0o600)
		assert.NilError(t, err)
		_ = ConvertToRaw(srcPath, destPath, &size, withBacking)
	})
}
