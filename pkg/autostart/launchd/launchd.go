// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package launchd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
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
	return launchctl(ctx, action, serviceTarget(instName))
}

func launchctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "launchctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Debugf("running command: %v", cmd.Args)
	return cmd.Run()
}

// AutoStartedServiceName returns the launchd service name if the instance is started by launchd.
func AutoStartedServiceName() string {
	// Assume the instance is started by launchd if XPC_SERVICE_NAME is set and not "0".
	// To confirm it is actually started by launchd, it needs to use `launch_activate_socket`.
	// But that requires actual socket activation setup in the plist file.
	// So we just check XPC_SERVICE_NAME here.
	if xpcServiceName := os.Getenv("XPC_SERVICE_NAME"); xpcServiceName != "0" {
		return xpcServiceName
	}
	return ""
}

var domainTarget = sync.OnceValue(func() string {
	return fmt.Sprintf("gui/%d", os.Getuid())
})

func serviceTarget(instName string) string {
	return fmt.Sprintf("%s/%s", domainTarget(), ServiceNameFrom(instName))
}

func RequestStart(ctx context.Context, inst *limatype.Instance) error {
	// If disabled, bootstrap will fail.
	_ = EnableDisableService(ctx, true, inst.Name)
	if err := launchctl(ctx, "bootstrap", domainTarget(), GetPlistPath(inst.Name)); err != nil {
		return fmt.Errorf("failed to start the instance %q via launchctl: %w", inst.Name, err)
	}
	return nil
}

func RequestStop(ctx context.Context, inst *limatype.Instance) (bool, error) {
	logrus.Debugf("AutoStartedIdentifier=%q, ServiceNameFrom=%q", inst.AutoStartedIdentifier, ServiceNameFrom(inst.Name))
	if inst.AutoStartedIdentifier == ServiceNameFrom(inst.Name) {
		logrus.Infof("Stopping the instance %q started by launchd", inst.Name)
		if err := launchctl(ctx, "bootout", serviceTarget(inst.Name)); err != nil {
			return false, fmt.Errorf("failed to stop the instance %q via launchctl: %w", inst.Name, err)
		}
		return true, nil
	}
	return false, nil
}
