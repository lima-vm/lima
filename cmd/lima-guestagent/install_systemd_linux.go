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
	"path/filepath"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/lima-vm/lima/v2/pkg/textutil"
)

func newInstallSystemdCommand() *cobra.Command {
	installSystemdCommand := &cobra.Command{
		Use:   "install-systemd",
		Short: "Install a systemd unit (user)",
		RunE:  installSystemdAction,
	}
	installSystemdCommand.Flags().Bool("guestagent-updated", false, "Indicate that the guest agent has been updated")
	installSystemdCommand.Flags().Int("vsock-port", 0, "Use vsock server on specified port")
	installSystemdCommand.Flags().String("virtio-port", "", "Use virtio server instead a UNIX socket")
	return installSystemdCommand
}

func installSystemdAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	guestAgentUpdated, err := cmd.Flags().GetBool("guestagent-updated")
	if err != nil {
		return err
	}
	vsockPort, err := cmd.Flags().GetInt("vsock-port")
	if err != nil {
		return err
	}
	virtioPort, err := cmd.Flags().GetString("virtio-port")
	if err != nil {
		return err
	}
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return err
	}
	unit, err := generateSystemdUnit(vsockPort, virtioPort, debug)
	if err != nil {
		return err
	}
	unitPath := "/etc/systemd/system/lima-guestagent.service"
	unitFileChanged := true
	if _, err := os.Stat(unitPath); !errors.Is(err, os.ErrNotExist) {
		if existingUnit, err := os.ReadFile(unitPath); err == nil && bytes.Equal(unit, existingUnit) {
			logrus.Infof("File %q is up-to-date", unitPath)
			unitFileChanged = false
		} else {
			logrus.Infof("File %q needs update", unitPath)
		}
	} else {
		unitDir := filepath.Dir(unitPath)
		if err := os.MkdirAll(unitDir, 0o755); err != nil {
			return err
		}
	}
	if unitFileChanged {
		if err := os.WriteFile(unitPath, unit, 0o644); err != nil {
			return err
		}
		logrus.Infof("Written file %q", unitPath)
	} else if !guestAgentUpdated {
		logrus.Info("lima-guestagent.service already up-to-date")
		return nil
	}
	// unitFileChanged || guestAgentUpdated
	args := make([][]string, 0, 4)
	if unitFileChanged {
		args = append(args, []string{"daemon-reload"})
	}
	args = slices.Concat(
		args,
		[][]string{
			{"enable", "lima-guestagent.service"},
			{"try-restart", "lima-guestagent.service"}, // try-restart: restart if running, otherwise do nothing
			{"start", "lima-guestagent.service"},       // start: start if not running, otherwise do nothing
		},
	)
	for _, args := range args {
		cmd := exec.CommandContext(ctx, "systemctl", append([]string{"--system"}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		logrus.Infof("Executing: %s", strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	logrus.Info("Done")
	return nil
}

//go:embed lima-guestagent.TEMPLATE.service
var systemdUnitTemplate string

func generateSystemdUnit(vsockPort int, virtioPort string, debug bool) ([]byte, error) {
	selfExeAbs, err := os.Executable()
	if err != nil {
		return nil, err
	}

	var args []string
	if vsockPort != 0 {
		args = append(args, fmt.Sprintf("--vsock-port %d", vsockPort))
	}
	if virtioPort != "" {
		args = append(args, fmt.Sprintf("--virtio-port %s", virtioPort))
	}
	if debug {
		args = append(args, "--debug")
	}

	m := map[string]string{
		"Binary": selfExeAbs,
		"Args":   strings.Join(args, " "),
	}
	return textutil.ExecuteTemplate(systemdUnitTemplate, m)
}
