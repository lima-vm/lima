//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package launchd

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetPlistPath(t *testing.T) {
	tests := []struct {
		Name         string
		InstanceName string
		Expected     string
	}{
		{
			Name:         "darwin with docker instance name",
			InstanceName: "docker",
			Expected:     "Library/LaunchAgents/io.lima-vm.autostart.docker.plist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Check(t, strings.HasSuffix(GetPlistPath(tt.InstanceName), tt.Expected))
		})
	}
}

func TestGetDaemonPlistPath(t *testing.T) {
	tests := []struct {
		Name         string
		InstanceName string
		Expected     string
	}{
		{
			Name:         "k3s instance",
			InstanceName: "k3s",
			Expected:     "/Library/LaunchDaemons/io.lima-vm.daemon.k3s.plist",
		},
		{
			Name:         "default instance",
			InstanceName: "default",
			Expected:     "/Library/LaunchDaemons/io.lima-vm.daemon.default.plist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Equal(t, GetDaemonPlistPath(tt.InstanceName), tt.Expected)
		})
	}
}

func TestDaemonServiceNameFrom(t *testing.T) {
	tests := []struct {
		Name         string
		InstanceName string
		Expected     string
	}{
		{
			Name:         "k3s instance",
			InstanceName: "k3s",
			Expected:     "io.lima-vm.daemon.k3s",
		},
		{
			Name:         "default instance",
			InstanceName: "default",
			Expected:     "io.lima-vm.daemon.default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Equal(t, DaemonServiceNameFrom(tt.InstanceName), tt.Expected)
		})
	}
}
