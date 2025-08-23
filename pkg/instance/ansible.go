// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package instance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func runAnsibleProvision(ctx context.Context, inst *limatype.Instance) error {
	for _, f := range inst.Config.Provision {
		if f.Mode == limatype.ProvisionModeAnsible {
			logrus.Infof("Waiting for ansible playbook %q", f.Playbook)
			if err := runAnsiblePlaybook(ctx, inst, f.Playbook); err != nil {
				return err
			}
		}
	}
	return nil
}

func runAnsiblePlaybook(ctx context.Context, inst *limatype.Instance, playbook string) error {
	inventory, err := createAnsibleInventory(inst)
	if err != nil {
		return err
	}
	logrus.Debugf("ansible-playbook -i %q %q", inventory, playbook)
	args := []string{"-i", inventory, playbook}
	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	cmd.Env = getAnsibleEnvironment(inst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createAnsibleInventory(inst *limatype.Instance) (string, error) {
	vars := map[string]any{
		"ansible_connection":      "ssh",
		"ansible_host":            inst.Hostname,
		"ansible_ssh_common_args": "-F " + inst.SSHConfigFile,
	}
	hosts := map[string]any{
		inst.Name: vars,
	}
	group := "lima"
	data := map[string]any{
		group: map[string]any{
			"hosts": hosts,
		},
	}
	bytes, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}
	inventory := filepath.Join(inst.Dir, filenames.AnsibleInventoryYAML)
	return inventory, os.WriteFile(inventory, bytes, 0o644)
}

func getAnsibleEnvironment(inst *limatype.Instance) []string {
	env := os.Environ()
	for key, val := range inst.Config.Param {
		env = append(env, fmt.Sprintf("PARAM_%s=%s", key, val))
	}
	return env
}
