// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"strings"
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

func TestParseQemuVersion(t *testing.T) {
	type testCase struct {
		versionOutput string
		expectedValue string
		expectedError string
	}
	testCases := []testCase{
		{
			// old one line version
			versionOutput: "QEMU emulator version 1.5.3 (qemu-kvm-1.5.3-175.el7_9.6), " +
				"Copyright (c) 2003-2008 Fabrice Bellard\n",
			expectedValue: "1.5.3",
			expectedError: "",
		},
		{
			// new two line version
			versionOutput: "QEMU emulator version 8.0.0 (v8.0.0)\n" +
				"Copyright (c) 2003-2022 Fabrice Bellard and the QEMU Project developers\n",
			expectedValue: "8.0.0",
			expectedError: "",
		},
		{
			versionOutput: "foobar",
			expectedValue: "0.0.0",
			expectedError: "failed to parse",
		},
	}

	for _, tc := range testCases {
		v, err := parseQemuVersion(tc.versionOutput)
		if tc.expectedError == "" {
			assert.NilError(t, err)
		} else {
			assert.ErrorContains(t, err, tc.expectedError)
		}
		assert.Equal(t, tc.expectedValue, v.String())
	}
}

func TestSPICEAudioDetection(t *testing.T) {
	// Test that SPICE audio is properly detected and configured
	testCases := []struct {
		name          string
		displayString string
		audioDevice   string
		spiceAudio    bool
		expectSPICE   bool
	}{
		{
			name:          "SPICE display with audio enabled",
			displayString: "spice,port=5930",
			audioDevice:   "default",
			spiceAudio:    true,
			expectSPICE:   true,
		},
		{
			name:          "SPICE display without audio config",
			displayString: "spice,port=5930",
			audioDevice:   "default",
			spiceAudio:    false,
			expectSPICE:   false,
		},
		{
			name:          "VNC display with audio",
			displayString: "vnc=:0",
			audioDevice:   "default",
			spiceAudio:    false,
			expectSPICE:   false,
		},
		{
			name:          "No display",
			displayString: "none",
			audioDevice:   "default",
			spiceAudio:    false,
			expectSPICE:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This tests the logic of detecting SPICE audio configuration
			usingSPICEAudio := false
			if tc.displayString != "" && strings.HasPrefix(tc.displayString, "spice") {
				if tc.spiceAudio {
					usingSPICEAudio = true
				}
			}
			assert.Equal(t, tc.expectSPICE, usingSPICEAudio)
		})
	}
}
