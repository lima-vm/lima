// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package toolset

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestReplaceExact(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		oldString string
		newString string
		expected  int
		want      string
		wantErr   string
	}{
		{
			name:      "single occurrence",
			content:   "foo\nbar\nbaz\n",
			oldString: "bar",
			newString: "BAR",
			expected:  1,
			want:      "foo\nBAR\nbaz\n",
		},
		{
			name:      "multiple occurrences",
			content:   "foo\nbar\nfoo\n",
			oldString: "foo",
			newString: "FOO",
			expected:  2,
			want:      "FOO\nbar\nFOO\n",
		},
		{
			name:      "multi-line old_string",
			content:   "a\nb\nc\nb\n",
			oldString: "b\nc\n",
			newString: "",
			expected:  1,
			want:      "a\nb\n",
		},
		{
			name:      "ambiguous",
			content:   "foo\nbar\nfoo\n",
			oldString: "foo",
			newString: "FOO",
			expected:  1,
			wantErr:   "expected 1 occurrence(s) of old_string, found 2",
		},
		{
			name:      "fewer occurrences than expected",
			content:   "foo\nbar\n",
			oldString: "foo",
			newString: "FOO",
			expected:  2,
			wantErr:   "expected 2 occurrence(s) of old_string, found 1",
		},
		{
			name:      "not found",
			content:   "foo\n",
			oldString: "bar",
			newString: "BAR",
			expected:  1,
			wantErr:   "old_string not found",
		},
		{
			name:      "empty old_string",
			content:   "foo\n",
			oldString: "",
			newString: "bar",
			expected:  1,
			wantErr:   "old_string must not be empty",
		},
		{
			name:      "identical strings",
			content:   "foo\n",
			oldString: "foo",
			newString: "foo",
			expected:  1,
			wantErr:   "old_string and new_string are identical",
		},
		{
			name:      "invalid expected_replacements",
			content:   "foo\n",
			oldString: "foo",
			newString: "bar",
			expected:  0,
			wantErr:   "expected_replacements must be at least 1, got 0",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := replaceExact(tc.content, tc.oldString, tc.newString, tc.expected)
			if tc.wantErr != "" {
				assert.Error(t, err, tc.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}
