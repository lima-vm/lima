// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package systemd

import (
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestGetUnitPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping testing on windows host")
	}
	tests := []struct {
		Name         string
		InstanceName string
		Expected     string
	}{
		{
			Name:         "linux with docker instance name",
			InstanceName: "docker",
			Expected:     ".config/systemd/user/lima-vm@docker.service",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			assert.Check(t, strings.HasSuffix(GetUnitPath(tt.InstanceName), tt.Expected))
		})
	}
}
