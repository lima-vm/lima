// Package autostart manage start at login unit files for darwin/linux
package autostart

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lima-vm/lima/pkg/textutil"
)

//go:embed lima-vm@INSTANCE.service
var systemdTemplate string

//go:embed io.lima-vm.autostart.INSTANCE.plist
var launchdTemplate string

// CreateStartAtLoginEntry respect host OS arch and create unit file.
func CreateStartAtLoginEntry(hostOS, instName, workDir string) error {
	unitPath := GetFilePath(hostOS, instName)
	if _, err := os.Stat(unitPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmpl, err := renderTemplate(hostOS, instName, workDir, os.Executable)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), os.ModePerm); err != nil {
		return err
	}
	if err := os.WriteFile(unitPath, tmpl, 0o644); err != nil {
		return err
	}
	return enableDisableService("enable", hostOS, GetFilePath(hostOS, instName))
}

// DeleteStartAtLoginEntry respect host OS arch and delete unit file.
// Return true, nil if unit file has been deleted.
func DeleteStartAtLoginEntry(hostOS, instName string) (bool, error) {
	unitPath := GetFilePath(hostOS, instName)
	if _, err := os.Stat(unitPath); err != nil {
		return false, err
	}
	if err := enableDisableService("disable", hostOS, GetFilePath(hostOS, instName)); err != nil {
		return false, err
	}
	if err := os.Remove(unitPath); err != nil {
		return false, err
	}
	return true, nil
}

// GetFilePath returns the path to autostart file with respect of host.
func GetFilePath(hostOS, instName string) string {
	var fileTmpl string
	if hostOS == "darwin" { // launchd plist
		fileTmpl = fmt.Sprintf("%s/Library/LaunchAgents/io.lima-vm.autostart.%s.plist", os.Getenv("HOME"), instName)
	}
	if hostOS == "linux" { // systemd service
		// Use instance name as argument to systemd service
		// Instance name available in unit file as %i
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome == "" {
			xdgConfigHome = filepath.Join(os.Getenv("HOME"), ".config")
		}
		fileTmpl = fmt.Sprintf("%s/systemd/user/lima-vm@%s.service", xdgConfigHome, instName)
	}
	return fileTmpl
}

func enableDisableService(action, hostOS, serviceWithPath string) error {
	// Get filename without extension
	filename := strings.TrimSuffix(path.Base(serviceWithPath), filepath.Ext(path.Base(serviceWithPath)))

	var args []string
	if hostOS == "darwin" {
		// man launchctl
		args = append(args, []string{
			"launchctl",
			action,
			fmt.Sprintf("gui/%s/%s", strconv.Itoa(os.Getuid()), filename),
		}...)
	} else {
		args = append(args, []string{
			"systemctl",
			"--user",
			action,
			filename,
		}...)
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func renderTemplate(hostOS, instName, workDir string, getExecutable func() (string, error)) ([]byte, error) {
	selfExeAbs, err := getExecutable()
	if err != nil {
		return nil, err
	}
	tmpToExecute := systemdTemplate
	if hostOS == "darwin" {
		tmpToExecute = launchdTemplate
	}
	return textutil.ExecuteTemplate(
		tmpToExecute,
		map[string]string{
			"Binary":   selfExeAbs,
			"Instance": instName,
			"WorkDir":  workDir,
		})
}
