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
	"testing"

	"github.com/lima-vm/lima/pkg/ptr"
	"gotest.tools/v3/assert"
)

func TestMarshalEmpty(t *testing.T) {
	_, err := Marshal(&LimaYAML{}, false)
	assert.NilError(t, err)
}

func TestMarshalTilde(t *testing.T) {
	y := LimaYAML{
		Mounts: []Mount{
			{Location: "~", Writable: ptr.Of(false)},
			{Location: "/tmp/lima", Writable: ptr.Of(true)},
			{Location: "null"},
		},
	}
	b, err := Marshal(&y, true)
	assert.NilError(t, err)
	// yaml will load ~ (or null) as null
	// make sure that it is always quoted
	assert.Equal(t, string(b), `---
images: []
mounts:
- location: "~"
  writable: false
- location: /tmp/lima
  writable: true
- location: "null"
...
`)
}
