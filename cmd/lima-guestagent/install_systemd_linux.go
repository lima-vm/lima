/*
Copyright The Lima Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/pkg/textutil"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newInstallSystemdCommand() *cobra.Command {
	installSystemdCommand := &cobra.Command{
		Use:   "install-systemd",
		Short: "install a systemd unit (user)",
		RunE:  installSystemdAction,
	}
	installSystemdCommand.Flags().Int("vsock-port", 0, "use vsock server on specified port")
	installSystemdCommand.Flags().String("virtio-port", "", "use virtio server instead a UNIX socket")
	return installSystemdCommand
}

func installSystemdAction(cmd *cobra.Command, _ []string) error {
	vsockPort, err := cmd.Flags().GetInt("vsock-port")
	if err != nil {
		return err
	}
	virtioPort, err := cmd.Flags().GetString("virtio-port")
	if err != nil {
		return err
	}
	unit, err := generateSystemdUnit(vsockPort, virtioPort)
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
		cmd := exec.Command("systemctl", append([]string{"--system"}, args...)...)
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

func generateSystemdUnit(vsockPort int, virtioPort string) ([]byte, error) {
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

	m := map[string]string{
		"Binary": selfExeAbs,
		"Args":   strings.Join(args, " "),
	}
	return textutil.ExecuteTemplate(systemdUnitTemplate, m)
}
