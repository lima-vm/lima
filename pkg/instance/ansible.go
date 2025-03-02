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

package instance

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/sirupsen/logrus"
)

func runAnsibleProvision(ctx context.Context, inst *store.Instance) error {
	for _, f := range inst.Config.Provision {
		if f.Mode == limayaml.ProvisionModeAnsible {
			logrus.Infof("Waiting for ansible playbook %q", f.Playbook)
			if err := runAnsiblePlaybook(ctx, inst, f.Playbook); err != nil {
				return err
			}
		}
	}
	return nil
}

func runAnsiblePlaybook(ctx context.Context, inst *store.Instance, playbook string) error {
	inventory, err := createAnsibleInventory(inst)
	if err != nil {
		return err
	}
	logrus.Debugf("ansible-playbook -i %q %q", inventory, playbook)
	args := []string{"-i", inventory, playbook}
	cmd := exec.CommandContext(ctx, "ansible-playbook", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func createAnsibleInventory(inst *store.Instance) (string, error) {
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
