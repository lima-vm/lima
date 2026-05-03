//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package blockdevice

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSudoers(t *testing.T) {
	sudoers, err := Sudoers("everyone")
	assert.NilError(t, err)
	exe, err := os.Executable()
	assert.NilError(t, err)
	assert.Equal(t, sudoers, "%everyone ALL=(root:wheel) NOPASSWD:NOSETENV: "+filepath.Clean(exe)+" "+SudoOpenBlockDeviceCommand+"\n")
}

func TestSudoOpenBlockDeviceRequestValidate(t *testing.T) {
	valid := sudoOpenBlockDeviceRequest{
		DevicePath: "/dev/disk4",
		SocketPath: "/tmp/block-device.0.sock",
	}
	assert.NilError(t, valid.validate())

	testCases := []struct {
		name          string
		request       sudoOpenBlockDeviceRequest
		errorContains string
	}{
		{
			name: "empty device path",
			request: sudoOpenBlockDeviceRequest{
				SocketPath: valid.SocketPath,
			},
			errorContains: "devicePath must not be empty",
		},
		{
			name: "relative device path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "disk4",
				SocketPath: valid.SocketPath,
			},
			errorContains: "must be an absolute path",
		},
		{
			name: "non dev device path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: "/tmp/disk4",
				SocketPath: valid.SocketPath,
			},
			errorContains: "must be under /dev",
		},
		{
			name: "unnormalized socket path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: "/tmp/../tmp/block-device.0.sock",
			},
			errorContains: "socketPath",
		},
		{
			name: "relative socket path",
			request: sudoOpenBlockDeviceRequest{
				DevicePath: valid.DevicePath,
				SocketPath: "block-device.0.sock",
			},
			errorContains: "must be an absolute path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.validate()
			assert.ErrorContains(t, err, tc.errorContains)
		})
	}
}
