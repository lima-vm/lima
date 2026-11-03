// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestLoadOrCreateInstance(t *testing.T) {
	tests := []struct {
		name            string
		yaml            string
		wantCPUs        int
		wantErrContains []string
	}{
		{
			name:     "valid configuration",
			yaml:     "images: [{location: /}]\ncpus: 2\n",
			wantCPUs: 2,
		},
		{
			name: "invalid configuration",
			yaml: "arch: [invalid\n",
			wantErrContains: []string{
				`instance "test" has configuration errors`,
				"failed to unmarshal YAML",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limaDir := t.TempDir()
			t.Setenv("LIMA_HOME", limaDir)
			instanceDir := filepath.Join(limaDir, "test")
			assert.NilError(t, os.MkdirAll(instanceDir, 0o700))
			assert.NilError(t, os.WriteFile(filepath.Join(instanceDir, filenames.LimaYAML), []byte(tt.yaml), 0o600))

			inst, err := loadOrCreateInstance(newTestStartCommand(t), []string{"test"}, false)
			if len(tt.wantErrContains) > 0 {
				for _, want := range tt.wantErrContains {
					assert.ErrorContains(t, err, want)
				}
				assert.Assert(t, inst == nil)
				return
			}

			assert.NilError(t, err)
			assert.Assert(t, inst != nil)
			assert.Assert(t, inst.Config != nil)
			assert.Equal(t, *inst.Config.CPUs, tt.wantCPUs)
		})
	}
}

func newTestStartCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := newStartCommand()
	cmd.Flags().Bool("tty", false, "")
	cmd.SetContext(t.Context())
	return cmd
}

// `--disk-path` relocates the disk of a new instance, so it must not be accepted
// by the commands that only operate on an existing instance.
func TestDiskPathFlagIsRegisteredForNewInstancesOnly(t *testing.T) {
	commands := map[string]*cobra.Command{
		"create":  newCreateCommand(),
		"start":   newStartCommand(),
		"edit":    newEditCommand(),
		"clone":   newCloneCommand(),
		"rename":  newRenameCommand(),
		"restart": newRestartCommand(),
	}
	for name, cmd := range commands {
		t.Run(name, func(t *testing.T) {
			flag := cmd.Flags().Lookup("disk-path")
			switch name {
			case "create", "start":
				assert.Assert(t, flag != nil, "`limactl %s` must accept --disk-path", name)
			default:
				assert.Assert(t, flag == nil, "`limactl %s` must not accept --disk-path", name)
			}
		})
	}
}

// removeStaleHostAgentFiles must not delete the PID file of a VM driver that runs as its
// own process: that driver may still be alive, and hiding it from the next inspection would
// let a second driver be started for the same instance.
func TestRemoveStaleHostAgentFilesKeepsSeparateDriver(t *testing.T) {
	dir := t.TempDir()
	driverPID := filepath.Join(dir, filenames.PIDFile("qemu"))
	for _, name := range []string{filenames.HostAgentPID, filenames.HostAgentSock, "qemu.sock"} {
		assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte("1"), 0o600))
	}
	assert.NilError(t, os.WriteFile(driverPID, []byte("222"), 0o600))

	removeStaleHostAgentFiles(&limatype.Instance{
		Dir: dir, VMType: "qemu", HostAgentPID: 111, DriverPID: 222,
	})

	for _, name := range []string{filenames.HostAgentPID, filenames.HostAgentSock} {
		_, err := os.Stat(filepath.Join(dir, name))
		assert.Assert(t, os.IsNotExist(err), "%s should have been removed", name)
	}
	_, err := os.Stat(driverPID)
	assert.NilError(t, err, "a separate driver's PID file must be kept")
	_, err = os.Stat(filepath.Join(dir, "qemu.sock"))
	assert.NilError(t, err, "a separate driver's socket must be kept")
}

// A driver that runs the VM inside the host agent process (such as vz) records the same PID,
// so its PID file refers to the departed host agent too and must be removed as well.
func TestRemoveStaleHostAgentFilesRemovesInProcessDriver(t *testing.T) {
	dir := t.TempDir()
	driverPID := filepath.Join(dir, filenames.PIDFile("vz"))
	for _, name := range []string{filenames.HostAgentPID, filenames.HostAgentSock} {
		assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte("111"), 0o600))
	}
	assert.NilError(t, os.WriteFile(driverPID, []byte("111"), 0o600))

	removeStaleHostAgentFiles(&limatype.Instance{
		Dir: dir, VMType: "vz", HostAgentPID: 111, DriverPID: 111,
	})

	_, err := os.Stat(driverPID)
	assert.Assert(t, os.IsNotExist(err), "an in-process driver's PID file should have been removed")
}

func TestLoadOrCreateInstanceRejectsDiskPathForExistingInstance(t *testing.T) {
	limaDir := t.TempDir()
	t.Setenv("LIMA_HOME", limaDir)
	instanceDir := filepath.Join(limaDir, "test")
	assert.NilError(t, os.MkdirAll(instanceDir, 0o700))
	assert.NilError(t, os.WriteFile(filepath.Join(instanceDir, filenames.LimaYAML), []byte("images: [{location: /}]\n"), 0o600))

	cmd := newTestStartCommand(t)
	assert.NilError(t, cmd.ParseFlags([]string{"--disk-path", filepath.Join(t.TempDir(), "test.img")}))

	inst, err := loadOrCreateInstance(cmd, []string{"test"}, false)
	assert.ErrorContains(t, err, "--disk-path")
	assert.ErrorContains(t, err, "can only be set when the instance is created")
	assert.Assert(t, inst == nil)

	// The existing instance must not get a disk symlink.
	_, err = os.Lstat(filepath.Join(instanceDir, filenames.Disk))
	assert.Assert(t, os.IsNotExist(err), "no disk symlink must be created for an existing instance")
}
