// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

func inspectStatus(instDir string, inst *Instance, y *limayaml.LimaYAML) {
	if inst.VMType == limayaml.AC {
		status, err := GetAcStatus(inst.Name)
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

type Network struct {
	Address  string `json:"address"`
	Gateway  string `json:"gateway"`
	Hostname string `json:"hostname"`
	Network  string `json:"network"`
}

type Container struct {
	Status   string         `json:"status"`
	Config   map[string]any `json:"configuration"`
	Networks []Network      `json:"networks,omitempty"`
}

// listContainers returns all containers in the system.
//
// but currently there is _no_ way to filter in the list.
// so we need to loop through all of them in the client.
func listContainers() ([]Container, error) {
	out, err := exec.Command(
		"container",
		"list",
		"--format=json",
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run `container list --format=json`, err: %w (out=%q)", err, out)
	}

	var list []Container
	err = json.Unmarshal(out, &list)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json output, err: %w (out=%q)", err, out)
	}
	return list, nil
}

func GetAcStatus(instName string) (string, error) {
	distroName := "lima-" + instName
	list, err := listContainers()
	if err != nil {
		return "", err
	}

	instState := StatusUninitialized
	for _, c := range list {
		// container don't have real ID
		// (any --name replaces the UUID)
		if c.Config["id"] == distroName {
			switch c.Status {
			case "stopped":
				instState = StatusStopped
			case "running":
				instState = StatusRunning
			default:
				instState = StatusUnknown
			}
			break
		}
	}

	return instState, nil
}

func GetSSHAddress(instName string) (string, error) {
	distroName := "lima-" + instName
	list, err := listContainers()
	if err != nil {
		return "", err
	}

	instAddress := "127.0.0.1"
	for _, c := range list {
		// container don't have real ID
		// (any --name replaces the UUID)
		if c.Config["id"] == distroName {
			if len(c.Networks) > 0 {
				instAddress = c.Networks[0].Address
			}
			break
		}
	}

	return strings.Replace(instAddress, "/24", "", 1), nil
}
