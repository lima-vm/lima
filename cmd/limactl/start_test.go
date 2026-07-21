// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

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
