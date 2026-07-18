// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package systemd

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

//go:embed lima-vm@INSTANCE.service
var Template string

// GetUnitPath returns the path to the systemd unit file for the given instance name.
func GetUnitPath(instName string) string {
	// Use instance name as argument to systemd service
	// Instance name available in unit file as %i
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return fmt.Sprintf("%s/systemd/user/%s", xdgConfigHome, UnitNameFrom(instName))
}

// UnitNameFrom returns the systemd service name for the given instance name.
func UnitNameFrom(instName string) string {
	return fmt.Sprintf("lima-vm@%s.service", instName)
}

// EnableDisableUnit enables or disables the systemd service for the given instance name.
func EnableDisableUnit(ctx context.Context, enable bool, instName string) error {
	action := "enable"
	if !enable {
		action = "disable"
	}
	return systemctl(ctx, "--user", action, UnitNameFrom(instName))
}

func systemctl(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logrus.Debugf("running command: %v", cmd.Args)
	return cmd.Run()
}

// AutoStartedUnitName returns the systemd service name if the instance is started by systemd.
func AutoStartedUnitName() string {
	return CurrentUnitName()
}

func RequestStart(ctx context.Context, inst *limatype.Instance) error {
	if err := systemctl(ctx, "--user", "start", UnitNameFrom(inst.Name)); err != nil {
		return fmt.Errorf("failed to start the instance %q via systemctl: %w", inst.Name, err)
	}
	return nil
}

func RequestStop(ctx context.Context, inst *limatype.Instance) (bool, error) {
	if inst.AutoStartedIdentifier == UnitNameFrom(inst.Name) {
		logrus.Infof("Stopping the instance %q started by systemd", inst.Name)
		if err := systemctl(ctx, "--user", "stop", inst.AutoStartedIdentifier); err != nil {
			return false, fmt.Errorf("failed to stop the instance %q via systemctl: %w", inst.Name, err)
		}
		return true, nil
	}
	return false, nil
}
