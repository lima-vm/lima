package osutil

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestCheckOrGetFreePort(t *testing.T) {
	type testCase struct {
		port       int
		expectedOK bool
	}

	testCases := []testCase{
		{
			// By default, macOS 53  are already used
			port:       53,
			expectedOK: false,
		},
		{
			port:       52,
			expectedOK: true,
		},
	}
	for _, tc := range testCases {
		respport := CheckOrGetFreePort(tc.port)
		assert.Equal(t, respport == tc.port, tc.expectedOK)
	}
}
