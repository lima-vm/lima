// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ptr

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestOf(t *testing.T) {
	assert.DeepEqual(t, bool(true), *Of(true))
	assert.DeepEqual(t, int(10), *Of(10))
	assert.DeepEqual(t, string(""), *Of(""))
	assert.DeepEqual(t, string("value"), *Of("value"))
}
