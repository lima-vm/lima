package main

import (
	_ "embed"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/templateutil"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var installSystemdCommand = &cli.Command{
	Name:   "install-systemd",
	Usage:  "install a systemd unit (user)",
	Action: installSystemdAction,
}

func installSystemdAction(clicontext *cli.Context) error {
	unit, err := generateSystemdUnit()
	if err != nil {
		return err
	}
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(unitPath); !errors.Is(err, os.ErrNotExist) {
		logrus.Infof("File %q already exists, overwriting", unitPath)
	} else {
		unitDir := filepath.Dir(unitPath)
		if err := os.MkdirAll(unitDir, 0755); err != nil {
			return err
		}
	}
	if err := os.WriteFile(unitPath, unit, 0644); err != nil {
		return err
	}
	logrus.Infof("Written file %q", unitPath)
	argss := [][]string{
		{"daemon-reload"},
		{"enable", "--now", "lima-guestagent.service"},
	}
	for _, args := range argss {
		cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
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

func systemdUnitPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	unitPath := filepath.Join(configDir, "systemd/user/lima-guestagent.service")
	return unitPath, nil
}

//go:embed lima-guestagent.TEMPLATE.service
var systemdUnitTemplate string

func generateSystemdUnit() ([]byte, error) {
	selfExeAbs, err := os.Executable()
	if err != nil {
		return nil, err
	}
	m := map[string]string{
		"Binary": selfExeAbs,
	}
	return templateutil.Execute(systemdUnitTemplate, m)
}
