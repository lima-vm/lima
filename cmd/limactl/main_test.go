// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestDefaultInstanceName(t *testing.T) {
	t.Run("returns default when LIMA_INSTANCE is unset", func(t *testing.T) {
		t.Setenv("LIMA_INSTANCE", "")
		assert.Equal(t, defaultInstanceName(), "default")
	})
	t.Run("respects LIMA_INSTANCE", func(t *testing.T) {
		t.Setenv("LIMA_INSTANCE", "myvm")
		assert.Equal(t, defaultInstanceName(), "myvm")
	})
}
