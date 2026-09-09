// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
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

func TestValidate9pMountOptions(t *testing.T) {
	tests := []struct {
		name    string
		nineP   limatype.NineP
		wantErr string
	}{
		{
			name: "valid non-default values",
			nineP: limatype.NineP{
				SecurityModel:   ptr.Of("mapped-xattr"),
				ProtocolVersion: ptr.Of("9p2000.u"),
				Cache:           ptr.Of("loose"),
			},
		},
		{
			// A comma in securityModel would inject extra options into the QEMU -virtfs argument.
			name:    "securityModel with injected option",
			nineP:   limatype.NineP{SecurityModel: ptr.Of("none,readonly=off")},
			wantErr: "mounts[0].9p.securityModel",
		},
		{
			name:    "unknown protocolVersion",
			nineP:   limatype.NineP{ProtocolVersion: ptr.Of("9p2000.L,foo=bar")},
			wantErr: "mounts[0].9p.protocolVersion",
		},
		{
			name:    "unknown cache",
			nineP:   limatype.NineP{Cache: ptr.Of("fscache,foo")},
			wantErr: "mounts[0].9p.cache",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &limatype.LimaYAML{
				Mounts: []limatype.Mount{{Location: "/", NineP: tt.nineP}},
			}
			err := validate9pMountOptions(cfg)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
