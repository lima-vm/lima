// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package launchd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
)

//go:embed io.lima-vm.autostart.INSTANCE.plist
var Template string

// GetPlistPath returns the path to the launchd plist file for the given instance name.
func GetPlistPath(instName string) string {
	return fmt.Sprintf("%s/Library/LaunchAgents/%s.plist", os.Getenv("HOME"), ServiceNameFrom(instName))
}

// ServiceNameFrom returns the launchd service name for the given instance name.
func ServiceNameFrom(instName string) string {
	return fmt.Sprintf("io.lima-vm.autostart.%s", instName)
}

// EnableDisableService enables or disables the launchd service for the given instance name.
func EnableDisableService(ctx context.Context, enable bool, instName string) error {
	action := "enable"
	if !enable {
		action = "disable"
	}
	return launchctl(ctx, action, fmt.Sprintf("gui/%d/%s", os.Getuid(), ServiceNameFrom(instName)))
}

func launchctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "launchctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Debugf("running command: %v", cmd.Args)
	return cmd.Run()
}
