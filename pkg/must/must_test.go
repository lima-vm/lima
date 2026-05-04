// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package must

import (
	"errors"
	"testing"

	"gotest.tools/v3/assert"
)

func TestMustSuccess(t *testing.T) {
	str := Must("string", nil)
	assert.Equal(t, str, "string")
}

func TestMustPanic(t *testing.T) {
	defer func() {
		r := recover()
		assert.Assert(t, r != nil, "Must should have panicked")
	}()

	Must("string", errors.New("test error"))
}
