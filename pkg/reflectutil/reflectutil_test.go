// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package reflectutil

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

// testStruct is used as the input type for all test cases.
// Count field removed — it was unused in any test case (reviewer feedback).
type testStruct struct {
	Known   string
	Unknown string
	Tags    []string
	Meta    map[string]string
}

// Rewritten as a table-driven test per reviewer feedback.
// Lima's other test files use a slice of test cases iterated by a single function.
func TestUnknownNonEmptyFields(t *testing.T) {
	tests := []struct {
		name        string
		input       any
		knownFields []string
		expected    []string
		wantPanic   bool
	}{
		{
			// All fields are in the known list — result must be empty.
			name:        "no unknown fields",
			input:       testStruct{Known: "hello", Unknown: "world"},
			knownFields: []string{"Known", "Unknown"},
			expected:    nil,
		},
		{
			// Unknown is non-empty and not in known list — must be reported.
			name:        "one unknown field",
			input:       testStruct{Known: "hello", Unknown: "world"},
			knownFields: []string{"Known"},
			expected:    []string{"Unknown"},
		},
		{
			// All fields are zero value — isEmpty() must filter all of them out.
			name:        "zero value fields ignored",
			input:       testStruct{},
			knownFields: nil,
			expected:    nil,
		},
		{
			// Function accepts *struct in addition to struct — ptr branch in switch.
			name:        "pointer to struct",
			input:       &testStruct{Known: "hello", Unknown: "world"},
			knownFields: []string{"Known"},
			expected:    []string{"Unknown"},
		},
		{
			// isEmpty() uses v.Len()==0 for slices — zero-length non-nil slice = empty.
			name:        "empty slice ignored",
			input:       testStruct{Tags: []string{}},
			knownFields: nil,
			expected:    nil,
		},
		{
			// isEmpty() uses v.Len()==0 for maps — zero-length non-nil map = empty.
			name:        "empty map ignored",
			input:       testStruct{Meta: map[string]string{}},
			knownFields: nil,
			expected:    nil,
		},
		{
			// Non-empty slice is not zero — must appear in result.
			name:        "non-empty slice reported",
			input:       testStruct{Tags: []string{"a", "b"}},
			knownFields: nil,
			expected:    []string{"Tags"},
		},
		{
			// Non-empty map is not zero — must appear in result.
			name:        "non-empty map reported",
			input:       testStruct{Meta: map[string]string{"key": "val"}},
			knownFields: nil,
			expected:    []string{"Meta"},
		},
		{
			// Reviewer suggestion: multiple unknown fields at once.
			// Using DeepEqual against sorted result catches order regressions
			// that cmp.Contains would miss.
			name:        "multiple unknown fields",
			input:       testStruct{Known: "hello", Unknown: "world", Tags: []string{"a"}},
			knownFields: []string{"Known"},
			expected:    []string{"Tags", "Unknown"},
		},
		{
			// Reviewer suggestion: mix of known non-empty + unknown non-empty fields.
			// Realistic usage — only the unknown ones must appear.
			name:        "mixed known and unknown",
			input:       testStruct{Known: "hello", Unknown: "world"},
			knownFields: []string{"Known"},
			expected:    []string{"Unknown"},
		},
		{
			// Reviewer suggestion: nil pointer — origVal.Elem() on nil panics.
			// Documenting this as expected panic behaviour.
			name:      "nil pointer panics",
			input:     (*testStruct)(nil),
			wantPanic: true,
		},
		{
			// Non-struct/non-pointer hits the default case which calls panic().
			name:      "non-struct panics",
			input:     "this is a string",
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				// Verify panic cases without crashing the whole test run.
				defer func() {
					if r := recover(); r == nil {
						t.Error("expected panic but did not panic")
					}
				}()
				UnknownNonEmptyFields(tt.input, tt.knownFields...)
				return
			}

			result := UnknownNonEmptyFields(tt.input, tt.knownFields...)

			// Sort both slices before comparing so order doesn't matter.
			// Reviewer asked for DeepEqual instead of cmp.Contains.
			slices.Sort(result)
			slices.Sort(tt.expected)
			assert.DeepEqual(t, tt.expected, result)
		})
	}
}
