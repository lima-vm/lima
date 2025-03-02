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

package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lima-vm/lima/pkg/store/filenames"
	"gotest.tools/v3/assert"
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
