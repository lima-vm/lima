// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package reflectutil

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

type testStruct struct {
	Known   string
	Unknown string
	Count   int
	Tags    []string
	Meta    map[string]string
}

func TestUnknownNonEmptyFields_NoUnknown(t *testing.T) {
	s := testStruct{Known: "hello", Unknown: "world"}
	result := UnknownNonEmptyFields(s, "Known", "Unknown")
	assert.Assert(t, cmp.Len(result, 0))
}

func TestUnknownNonEmptyFields_WithUnknown(t *testing.T) {
	s := testStruct{Known: "hello", Unknown: "world"}
	result := UnknownNonEmptyFields(s, "Known")
	assert.Assert(t, cmp.Contains(result, "Unknown"))
}

func TestUnknownNonEmptyFields_ZeroValueIgnored(t *testing.T) {
	// All fields are zero — none should appear even with no known fields listed
	s := testStruct{}
	result := UnknownNonEmptyFields(s)
	assert.Assert(t, cmp.Len(result, 0))
}

func TestUnknownNonEmptyFields_PointerToStruct(t *testing.T) {
	s := &testStruct{Known: "hello", Unknown: "world"}
	result := UnknownNonEmptyFields(s, "Known")
	assert.Assert(t, cmp.Contains(result, "Unknown"))
}

func TestUnknownNonEmptyFields_EmptySliceIgnored(t *testing.T) {
	// isEmpty() explicitly handles zero-length slice via v.Len() == 0
	s := testStruct{Tags: []string{}}
	result := UnknownNonEmptyFields(s)
	assert.Assert(t, cmp.Len(result, 0))
}

func TestUnknownNonEmptyFields_EmptyMapIgnored(t *testing.T) {
	// isEmpty() explicitly handles zero-length map via v.Len() == 0
	s := testStruct{Meta: map[string]string{}}
	result := UnknownNonEmptyFields(s)
	assert.Assert(t, cmp.Len(result, 0))
}

func TestUnknownNonEmptyFields_NonEmptySliceReported(t *testing.T) {
	s := testStruct{Tags: []string{"a", "b"}}
	result := UnknownNonEmptyFields(s)
	assert.Assert(t, cmp.Contains(result, "Tags"))
}

func TestUnknownNonEmptyFields_NonEmptyMapReported(t *testing.T) {
	s := testStruct{Meta: map[string]string{"key": "value"}}
	result := UnknownNonEmptyFields(s)
	assert.Assert(t, cmp.Contains(result, "Meta"))
}

func TestUnknownNonEmptyFields_PanicsOnNonStruct(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for non-struct input, but did not panic")
		}
	}()
	UnknownNonEmptyFields("this is a string")
}