package qemu

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestArgValue(t *testing.T) {
	type testCase struct {
		key           string
		expectedValue string
		expectedOK    bool
	}
	args := []string{"-cpu", "foo", "-no-reboot", "-m", "2G", "-s"}
	testCases := []testCase{
		{
			key:           "-cpu",
			expectedValue: "foo",
			expectedOK:    true,
		},
		{
			key:           "-no-reboot",
			expectedValue: "",
			expectedOK:    true,
		},
		{
			key:           "-m",
			expectedValue: "2G",
			expectedOK:    true,
		},
		{
			key:           "-machine",
			expectedValue: "",
			expectedOK:    false,
		},
		{
			key:           "-s",
			expectedValue: "",
			expectedOK:    true,
		},
	}

	for _, tc := range testCases {
		v, ok := argValue(args, tc.key)
		assert.Equal(t, tc.expectedValue, v)
		assert.Equal(t, tc.expectedOK, ok)
	}
}
