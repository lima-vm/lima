//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/autostart/launchd"
)

// TestPlistExitTimeOutExceedsGracefulStopTimeout guards the stop budget of autostarted
// instances. launchd SIGKILLs a job ExitTimeOut seconds after SIGTERM, and the VM runs inside
// the host agent, so ExitTimeOut must leave Stop the full gracefulStopTimeout to wait for the
// guest to power off.
func TestPlistExitTimeOutExceedsGracefulStopTimeout(t *testing.T) {
	re := regexp.MustCompile(`<key>ExitTimeOut</key>\s*<integer>(\d+)</integer>`)
	for name, tmpl := range map[string]string{
		"LaunchAgent":  launchd.Template,
		"LaunchDaemon": launchd.DaemonTemplate,
	} {
		t.Run(name, func(t *testing.T) {
			m := re.FindStringSubmatch(tmpl)
			assert.Assert(t, m != nil, "template has no ExitTimeOut")
			sec, err := strconv.Atoi(m[1])
			assert.NilError(t, err)
			exitTimeOut := time.Duration(sec) * time.Second
			assert.Assert(t, exitTimeOut > gracefulStopTimeout,
				"ExitTimeOut %v must exceed gracefulStopTimeout %v", exitTimeOut, gracefulStopTimeout)
		})
	}
}
