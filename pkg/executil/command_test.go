// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package executil

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
)

func TestWithContext(t *testing.T) {
	ctx := context.Background()
	var o options
	opt := WithContext(ctx)
	err := opt(&o)
	assert.NilError(t, err)
	assert.Equal(t, o.ctx, ctx)
}

func TestWithContext_NilByDefault(t *testing.T) {
	var o options
	assert.Assert(t, o.ctx == nil)
}