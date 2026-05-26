// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// From https://github.com/containerd/containerd/blob/v2.1.1/pkg/identifiers/validate_test.go
// SPDX-FileCopyrightText: Copyright The containerd Authors
// LICENSE: https://github.com/containerd/containerd/blob/v2.1.1/LICENSE
// NOTICE: https://github.com/containerd/containerd/blob/v2.1.1/NOTICE

/*
   Copyright The containerd Authors.

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

package identifiers

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidIdentifiers(t *testing.T) {
	for _, input := range []string{
		"default",
		"Default",
		t.Name(),
		"default-default",
		"containerd.io",
		"foo.boo",
		"swarmkit.docker.io",
		"0912341234",
		"task.0.0123456789",
		"container.system-75-f19a.00",
		"underscores_are_allowed",
		strings.Repeat("a", maxLength),
	} {
		t.Run(input, func(t *testing.T) {
			assert.NilError(t, Validate(input))
		})
	}
}

func TestInvalidIdentifiers(t *testing.T) {
	for _, input := range []string{
		"",
		".foo..foo",
		"foo/foo",
		"foo/..",
		"foo..foo",
		"foo.-boo",
		"-foo.boo",
		"foo.boo-",
		"but__only_tasteful_underscores",
		"zn--e9.org", // or something like it!
		"default--default",
		strings.Repeat("a", maxLength+1),
	} {
		t.Run(input, func(t *testing.T) {
			assert.ErrorContains(t, Validate(input), "")
		})
	}
}
