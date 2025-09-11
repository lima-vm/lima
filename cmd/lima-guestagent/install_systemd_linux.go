// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	installSystemdCommand.Flags().Int("vsock-port", 0, "Use vsock server on specified port")
	installSystemdCommand.Flags().String("virtio-port", "", "Use virtio server instead a UNIX socket")
	installSystemdCommand.Flags().StringSlice("docker-sockets", []string{}, "Paths to Docker socket files to monitor for exposed ports")
	installSystemdCommand.Flags().StringSlice("containerd-sockets", []string{}, "Paths to Containerd socket files to monitor for exposed ports")
	installSystemdCommand.Flags().StringSlice("kubernetes-configs", []string{}, "Path to Kubernetes config files to monitor for ports")
	return installSystemdCommand
}

func installSystemdAction(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
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
	dockerSockets, err := cmd.Flags().GetStringSlice("docker-sockets")
	if err != nil {
		return err
	}
	containerdSockets, err := cmd.Flags().GetStringSlice("containerd-sockets")
	if err != nil {
		return err
	}
	kubernetesConfigs, err := cmd.Flags().GetStringSlice("kubernetes-configs")
	if err != nil {
		return err
	}
	unit, err := generateSystemdUnit(
		vsockPort,
		virtioPort,
		dockerSockets,
		containerdSockets,
		kubernetesConfigs,
		debug)
	if err != nil {
		return err
	}
	unitPath := "/etc/systemd/system/lima-guestagent.service"
	if _, err := os.Stat(unitPath); !errors.Is(err, os.ErrNotExist) {
		logrus.Infof("File %q already exists, overwriting", unitPath)
	} else {
		unitDir := filepath.Dir(unitPath)
		if err := os.MkdirAll(unitDir, 0o755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(unitPath, unit, 0o644); err != nil {
		return err
	}
	logrus.Infof("Written file %q", unitPath)
	args := [][]string{
		{"daemon-reload"},
		{"enable", "lima-guestagent.service"},
		{"start", "lima-guestagent.service"},
		{"try-restart", "lima-guestagent.service"},
	}
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

func generateSystemdUnit(vsockPort int, virtioPort string, dockerSockets, containerdSockets, kubeConfigs []string, debug bool) ([]byte, error) {
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
	if len(dockerSockets) > 0 {
		args = append(args, "--docker-sockets", strings.Join(dockerSockets, ","))
	}
	if len(containerdSockets) > 0 {
		args = append(args, "--containerd-sockets", strings.Join(containerdSockets, ","))
	}
	if len(kubeConfigs) > 0 {
		args = append(args, "--kubernetes-configs", strings.Join(kubeConfigs, ","))
	}

	m := map[string]string{
		"Binary": selfExeAbs,
		"Args":   strings.Join(args, " "),
	}
	return textutil.ExecuteTemplate(systemdUnitTemplate, m)
}
