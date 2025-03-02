/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
