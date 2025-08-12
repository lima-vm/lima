// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

func inspectStatus(instDir string, inst *Instance, y *limayaml.LimaYAML) {
	if inst.VMType == limayaml.DC {
		status, err := GetDcStatus(inst.Name)
		if err != nil {
			inst.Status = StatusBroken
			inst.Errors = append(inst.Errors, err)
		} else {
			inst.Status = status
		}

		inst.SSHLocalPort = 22

		if inst.Status == StatusRunning {
			sshAddr, err := GetSSHAddress(inst.Name)
			if err == nil {
				inst.SSHAddress = sshAddr
			} else {
				inst.Errors = append(inst.Errors, err)
			}
		}
	} else {
		inspectStatusWithPIDFiles(instDir, inst, y)
	}
}

func inspectContainer(distroName, format string) (string, error) {
	out, err := exec.Command(
		"docker",
		"inspect",
		"--format="+format,
		distroName,
	).CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), fmt.Sprintf("No such object: %s", distroName)) {
			return "", nil
		}
		if strings.Contains(string(out), "map has no entry for key") {
			return "", nil
		}
		return "", fmt.Errorf("failed to run `docker inspect`, err: %w (out=%q)", err, out)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

func GetDcStatus(instName string) (string, error) {
	distroName := "lima-" + instName
	out, err := inspectContainer(distroName, "{{ .State.Status }}")
	if err != nil {
		return "", err
	}
	if out == "" {
		return StatusUninitialized, nil
	}

	var instState string
	switch out {
	case "exited":
		instState = StatusStopped
	case "running":
		instState = StatusRunning
	default:
		instState = StatusUnknown
	}

	return instState, nil
}

func GetSSHAddress(instName string) (string, error) {
	distroName := "lima-" + instName
	out, err := inspectContainer(distroName, "{{ .NetworkSettings.IPAddress }}")
	if err != nil {
		return "", err
	}
	if out == "" {
		return "127.0.0.1", nil
	}

	instAddress := out

	return instAddress, nil
}
