package smbpasswd

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestNTHash(t *testing.T) {
	testCases := map[string]string{
		"foobar": "BAAC3929FABC9E6DCD32421BA94A84D4",
	}

	for plain, expected := range testCases {
		got, err := NTHash(plain)
		t.Logf("plain=%q, expected=%q, got=%q", plain, expected, got)
		assert.NilError(t, err)
		assert.Equal(t, expected, got)
	}
}

func TestSMBPasswd(t *testing.T) {
	type testCase struct {
		username      string
		uid           int
		plainPassword string
		lct           time.Time
		expected      string
	}

	testCases := []testCase{
		{
			username:      "foo",
			uid:           501,
			plainPassword: "foobar",
			lct:           time.Date(2021, time.January, 1, 00, 0, 0, 0, time.UTC),
			expected:      "foo:501:XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX:BAAC3929FABC9E6DCD32421BA94A84D4:[U          ]:LCT-5FEE6600:",
		},
	}

	for _, tc := range testCases {
		got, err := SMBPasswd(tc.username, tc.uid, tc.plainPassword, tc.lct)
		t.Logf("tc=%+v, got=%q", tc, got)
		assert.NilError(t, err)
		assert.Equal(t, tc.expected, got)
	}
}
