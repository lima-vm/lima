// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/textutil"
)

func newInstallLaunchdCommand() *cobra.Command {
	installLaunchdCommand := &cobra.Command{
		Use:   "install-launchd",
		Short: "Install a launchd LaunchDaemon",
		RunE:  installLaunchdAction,
	}
	installLaunchdCommand.Flags().Bool("guestagent-updated", false, "Indicate that the guest agent has been updated")
	installLaunchdCommand.Flags().Int("vsock-port", 0, "Use vsock server on specified port")
	return installLaunchdCommand
}

func installLaunchdAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	guestAgentUpdated, err := cmd.Flags().GetBool("guestagent-updated")
	if err != nil {
		return err
	}
	vsockPort, err := cmd.Flags().GetInt("vsock-port")
	if err != nil {
		return err
	}
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}
	plist, err := generateLaunchdPlist(vsockPort, debug)
	if err != nil {
		return err
	}
	plistPath := "/Library/LaunchDaemons/io.lima-vm.lima-guestagent.plist"
	plistFileChanged := true
	if _, err := os.Stat(plistPath); !errors.Is(err, os.ErrNotExist) {
		if existingPlist, err := os.ReadFile(plistPath); err == nil && bytes.Equal(plist, existingPlist) {
			logrus.Infof("File %q is up-to-date", plistPath)
			plistFileChanged = false
		} else {
			logrus.Infof("File %q needs update", plistPath)
		}
	}
	if plistFileChanged {
		if err := os.WriteFile(plistPath, plist, 0o644); err != nil {
			return err
		}
		logrus.Infof("Written file %q", plistPath)
	} else if !guestAgentUpdated {
		logrus.Info("io.lima-vm.lima-guestagent already up-to-date")
		return nil
	}
	// plistFileChanged || guestAgentUpdated
	// Unload existing service (ignore errors if not currently loaded)
	unloadCmd := exec.CommandContext(ctx, "launchctl", "unload", plistPath)
	logrus.Infof("Executing: %s", strings.Join(unloadCmd.Args, " "))
	if err := unloadCmd.Run(); err != nil {
		logrus.Debugf("launchctl unload (expected on first install): %v", err)
	}

	loadCmd := exec.CommandContext(ctx, "launchctl", "load", "-w", plistPath)
	loadCmd.Stdout = os.Stdout
	loadCmd.Stderr = os.Stderr
	logrus.Infof("Executing: %s", strings.Join(loadCmd.Args, " "))
	if err := loadCmd.Run(); err != nil {
		return err
	}
	logrus.Info("Done")
	return nil
}

//go:embed lima-guestagent.TEMPLATE.plist
var launchdPlistTemplate string

func generateLaunchdPlist(vsockPort int, debug bool) ([]byte, error) {
	selfExeAbs, err := os.Executable()
	if err != nil {
		return nil, err
	}

	var extraArgs []string
	if vsockPort != 0 {
		extraArgs = append(extraArgs, "--vsock-port", fmt.Sprintf("%d", vsockPort))
	}
	if debug {
		extraArgs = append(extraArgs, "--debug")
	}

	m := map[string]any{
		"Binary":    selfExeAbs,
		"ExtraArgs": extraArgs,
	}
	return textutil.ExecuteTemplate(launchdPlistTemplate, m)
}
