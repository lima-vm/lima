// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ioutilx

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestReadAtMaximum(t *testing.T) {
	t.Run("below limit", func(t *testing.T) {
		b, err := ReadAtMaximum(strings.NewReader("abc"), 4)
		assert.NilError(t, err)
		assert.Equal(t, string(b), "abc")
	})
	t.Run("at limit", func(t *testing.T) {
		b, err := ReadAtMaximum(strings.NewReader("abcd"), 4)
		assert.NilError(t, err)
		assert.Equal(t, string(b), "abcd")
	})
	t.Run("above limit", func(t *testing.T) {
		_, err := ReadAtMaximum(strings.NewReader("abcde"), 4)
		assert.ErrorContains(t, err, "exceeded the limit (4 bytes)")
	})
}
