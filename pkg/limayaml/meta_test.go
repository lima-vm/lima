// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestFillDefaultMeta(t *testing.T) {
	tests := []struct {
		name     string
		d, y, o  limatype.Meta
		expected limatype.Meta
	}{
		{name: "unset stays unset"},
		{
			name:     "default.yaml is used when lima.yaml has no meta",
			d:        limatype.Meta{"group": "g"},
			expected: limatype.Meta{"group": "g"},
		},
		{
			name: "maps are merged recursively in default, lima, override order",
			d: limatype.Meta{
				"owner": map[string]any{"manager": "x", "locked": []any{"delete"}},
				"group": "g",
			},
			y: limatype.Meta{"owner": map[string]any{"locked": []any{"edit"}}},
			o: limatype.Meta{"owner": map[string]any{"hint": "h"}},
			expected: limatype.Meta{
				"owner": map[string]any{"manager": "x", "locked": []any{"edit"}, "hint": "h"},
				"group": "g",
			},
		},
		{
			name:     "lists are replaced, not appended",
			d:        limatype.Meta{"tags": []any{"a", "b"}},
			y:        limatype.Meta{"tags": []any{"b", "c"}},
			expected: limatype.Meta{"tags": []any{"b", "c"}},
		},
		{
			name:     "a list replaces a map",
			d:        limatype.Meta{"k": map[string]any{"x": 1}},
			y:        limatype.Meta{"k": []any{1, 2}},
			expected: limatype.Meta{"k": []any{1, 2}},
		},
		{
			name:     "a map replaces a scalar",
			y:        limatype.Meta{"k": "s"},
			o:        limatype.Meta{"k": map[string]any{"x": 1}},
			expected: limatype.Meta{"k": map[string]any{"x": 1}},
		},
		{
			name:     "a scalar replaces a map",
			y:        limatype.Meta{"k": map[string]any{"x": 1}},
			o:        limatype.Meta{"k": "s"},
			expected: limatype.Meta{"k": "s"},
		},
		{
			name:     "null replaces a value",
			d:        limatype.Meta{"k": map[string]any{"x": 1}},
			y:        limatype.Meta{"k": nil},
			expected: limatype.Meta{"k": nil},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			y := limatype.LimaYAML{Meta: tc.y}
			d := limatype.LimaYAML{Meta: tc.d}
			o := limatype.LimaYAML{Meta: tc.o}
			FillDefault(t.Context(), &y, &d, &o, "test.yaml", false)
			assert.DeepEqual(t, y.Meta, tc.expected)
		})
	}
}

func TestFillDefaultMetaDoesNotModifyInputs(t *testing.T) {
	d := limatype.LimaYAML{Meta: limatype.Meta{"owner": map[string]any{"manager": "x"}}}
	y := limatype.LimaYAML{Meta: limatype.Meta{"owner": map[string]any{"hint": "h"}}}
	o := limatype.LimaYAML{}
	FillDefault(t.Context(), &y, &d, &o, "test.yaml", false)

	assert.DeepEqual(t, d.Meta, limatype.Meta{"owner": map[string]any{"manager": "x"}})
	y.Meta["owner"].(map[string]any)["manager"] = "changed"
	assert.DeepEqual(t, d.Meta, limatype.Meta{"owner": map[string]any{"manager": "x"}})
}
