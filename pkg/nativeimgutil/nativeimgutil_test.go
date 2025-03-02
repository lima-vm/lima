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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func createImg(name, format, size string) error {
	return exec.Command("qemu-img", "create", name, "-f", format, size).Run()
}

func TestConvertToRaw(t *testing.T) {
	_, err := exec.LookPath("qemu-img")
	if err != nil {
		t.Skipf("qemu-img does not seem installed: %v", err)
	}
	tmpDir := t.TempDir()

	qcowImage, err := os.Create(filepath.Join(tmpDir, "qcow.img"))
	assert.NilError(t, err)
	defer qcowImage.Close()
	err = createImg(qcowImage.Name(), "qcow2", "1M")
	assert.NilError(t, err)

	rawImage, err := os.Create(filepath.Join(tmpDir, "raw.img"))
	assert.NilError(t, err)
	defer rawImage.Close()
	err = createImg(rawImage.Name(), "raw", "1M")
	assert.NilError(t, err)

	rawImageExtended, err := os.Create(filepath.Join(tmpDir, "raw_extended.img"))
	assert.NilError(t, err)
	defer rawImageExtended.Close()
	err = createImg(rawImageExtended.Name(), "raw", "2M")
	assert.NilError(t, err)

	t.Run("qcow without backing file", func(t *testing.T) {
		resultImage := filepath.Join(tmpDir, strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
		assert.NilError(t, err)

		err = ConvertToRaw(qcowImage.Name(), resultImage, nil, false)
		assert.NilError(t, err)
		assertFileEquals(t, rawImage.Name(), resultImage)
	})

	t.Run("qcow with backing file", func(t *testing.T) {
		resultImage := filepath.Join(tmpDir, strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
		assert.NilError(t, err)

		err = ConvertToRaw(qcowImage.Name(), resultImage, nil, true)
		assert.NilError(t, err)
		assertFileEquals(t, rawImage.Name(), resultImage)
	})

	t.Run("qcow with extra size", func(t *testing.T) {
		resultImage := filepath.Join(tmpDir, strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
		assert.NilError(t, err)
		size := int64(2_097_152) // 2mb
		err = ConvertToRaw(qcowImage.Name(), resultImage, &size, false)
		assert.NilError(t, err)
		assertFileEquals(t, rawImageExtended.Name(), resultImage)
	})

	t.Run("raw", func(t *testing.T) {
		resultImage := filepath.Join(tmpDir, strings.ReplaceAll(t.Name(), string(os.PathSeparator), "_"))
		assert.NilError(t, err)

		err = ConvertToRaw(rawImage.Name(), resultImage, nil, false)
		assert.NilError(t, err)
		assertFileEquals(t, rawImage.Name(), resultImage)
	})
}

func assertFileEquals(t *testing.T, expected, actual string) {
	expectedContent, err := os.ReadFile(expected)
	assert.NilError(t, err)
	actualContent, err := os.ReadFile(actual)
	assert.NilError(t, err)
	assert.DeepEqual(t, expectedContent, actualContent)
}
