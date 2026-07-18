// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package yqutil

import (
	"testing"
)

func FuzzEvaluateExpression(f *testing.F) {
	f.Fuzz(func(t *testing.T, expression string, content []byte) {
		_, _ = EvaluateExpression(t.Context(), expression, content)
	})
}
