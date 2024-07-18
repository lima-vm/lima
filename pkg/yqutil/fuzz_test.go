package yqutil

import (
	"testing"
)

func FuzzEvaluateExpression(f *testing.F) {
	f.Fuzz(func(_ *testing.T, expression string, content []byte) {
		_, _ = EvaluateExpression(expression, content)
	})
}
