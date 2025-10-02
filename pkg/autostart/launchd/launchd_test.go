// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package launchd

import (
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetPlistPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping testing on windows host")
	}
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
