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

package limayaml

import (
	"encoding/json"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func dumpJSON(t *testing.T, d any) string {
	b, err := json.Marshal(d)
	assert.NilError(t, err)
	return string(b)
}

const emptyYAML = "images: []\n"

func TestEmptyYAML(t *testing.T) {
	var y LimaYAML
	t.Log(dumpJSON(t, y))
	b, err := Marshal(&y, false)
	assert.NilError(t, err)
	assert.Equal(t, string(b), emptyYAML)
}

const defaultYAML = "images: []\n"

func TestDefaultYAML(t *testing.T) {
	bytes, err := os.ReadFile("default.yaml")
	assert.NilError(t, err)
	var y LimaYAML
	err = Unmarshal(bytes, &y, "")
	assert.NilError(t, err)
	y.Images = nil                // remove default images
	y.Mounts = nil                // remove default mounts
	y.MountTypesUnsupported = nil // remove default workaround for kernel 6.9-6.11
	t.Log(dumpJSON(t, y))
	b, err := Marshal(&y, false)
	assert.NilError(t, err)
	assert.Equal(t, string(b), defaultYAML)
}
