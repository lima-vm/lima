// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
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

func TestQemuExtraArgs(t *testing.T) {
	type testCase struct {
		name          string
		inputYAML     string
		expected      []string
		expectedError string
	}
	testCases := []testCase{
		{
			name: "no vmOpts",
			inputYAML: `
vmType: "qemu"
`,
			expected: nil,
		},
		{
			name: "qemu opts without extra args",
			inputYAML: `
vmType: "qemu"
vmOpts:
  qemu:
    minimumVersion: "8.2.1"
`,
			expected: nil,
		},
		{
			name: "extra args set",
			inputYAML: `
vmType: "qemu"
vmOpts:
  qemu:
    extraArgs:
    - "-device"
    - "virtio-balloon"
    - "-overcommit"
    - "mem-lock=off"
`,
			expected: []string{"-device", "virtio-balloon", "-overcommit", "mem-lock=off"},
		},
		{
			name: "structurally wrong extra args",
			inputYAML: `
vmType: "qemu"
vmOpts:
  qemu:
    extraArgs:
    - "-device"
    - {bad: "shape"}
`,
			expectedError: "failed to convert `vmOpts.qemu.extraArgs`",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var y limatype.LimaYAML
			err := limayaml.Unmarshal([]byte(tc.inputYAML), &y, "lima.yaml")
			assert.NilError(t, err)

			got, err := qemuExtraArgs(&y)
			if tc.expectedError == "" {
				assert.NilError(t, err)
				assert.DeepEqual(t, got, tc.expected)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}
