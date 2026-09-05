// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/coreos/go-semver/semver"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/ptr"
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

func TestSwtpmCmdline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("swtpm unix socket mode is not supported on Windows host")
	}

	tmpDir := t.TempDir()

	// Create a mock swtpm binary.
	binDir := filepath.Join(tmpDir, "bin")
	err := os.MkdirAll(binDir, 0o755)
	assert.NilError(t, err)
	swtpmPath := filepath.Join(binDir, "swtpm")
	err = os.WriteFile(swtpmPath, []byte{}, 0o755)
	assert.NilError(t, err)

	// Overwrite PATH so that the function find the mock binary.
	t.Setenv("PATH", binDir)

	// Setup configs and expected value
	cfg := Config{
		Name:        "tpm-test",
		InstanceDir: tmpDir,
		LimaYAML:    &limatype.LimaYAML{},
	}

	stateDir := filepath.Join(tmpDir, filenames.SwtpmDir)
	swtpmSock := filepath.Join(tmpDir, filenames.SwtpmSock)

	expectedArgs := []string{
		"socket",
		"--tpmstate", "dir=" + stateDir,
		"--ctrl", "type=unixio,path=" + swtpmSock,
		"--tpm2",
		"--terminate",
		"--log", "level=1",
	}

	exe, args, err := SwtpmCmdline(cfg)
	assert.NilError(t, err)
	assert.Equal(t, exe, swtpmPath)
	assert.DeepEqual(t, args, expectedArgs)

	// Verify that state directory was created.
	_, err = os.Stat(stateDir)
	assert.NilError(t, err)

	// Verify that stale socket is removed.
	err = os.WriteFile(swtpmSock, []byte("stale socket"), 0o644)
	assert.NilError(t, err)
	// Call again to clean up the stale socket.
	_, _, err = SwtpmCmdline(cfg)
	assert.NilError(t, err)
	_, err = os.Stat(swtpmSock)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestParseKVMNestedParam(t *testing.T) {
	type testCase struct {
		value         string
		expected      bool
		expectedError string
	}
	testCases := []testCase{
		{value: "Y\n", expected: true},  // kvm_intel (bool)
		{value: "N\n", expected: false}, // kvm_intel (bool)
		{value: "1\n", expected: true},  // kvm_amd (int)
		{value: "0\n", expected: false}, // kvm_amd (int)
		{value: "", expectedError: "unexpected value"},
		{value: "foo", expectedError: "unexpected value"},
	}
	for _, tc := range testCases {
		enabled, err := parseKVMNestedParam(tc.value)
		if tc.expectedError == "" {
			assert.NilError(t, err)
		} else {
			assert.ErrorContains(t, err, tc.expectedError)
		}
		assert.Equal(t, tc.expected, enabled)
	}
}

func TestValidateConfigNestedVirtualization(t *testing.T) {
	cfg := &limatype.LimaYAML{
		NestedVirtualization: ptr.Of(true),
	}
	err := validateConfig(cfg)
	if runtime.GOOS == "darwin" {
		productVer, verErr := osutil.ProductVersion()
		assert.NilError(t, verErr)
		if productVer.LessThan(*semver.New("15.0.0")) {
			assert.ErrorContains(t, err, "nested virtualization requires macOS 15 or newer")
		} else {
			assert.NilError(t, err)
		}
	} else {
		assert.NilError(t, err)
	}
}
