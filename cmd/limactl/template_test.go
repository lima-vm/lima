// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

const paramTemplate = `images:
- location: /etc/profile
mounts:
- location: "{{.Param.Dir}}"
param:
  Dir:
`

func runTemplateCommand(t *testing.T, cmd *cobra.Command, args ...string) error {
	t.Helper()
	t.Setenv("LIMA_HOME", t.TempDir())
	tmplPath := filepath.Join(t.TempDir(), "param.yaml")
	assert.NilError(t, os.WriteFile(tmplPath, []byte(paramTemplate), 0o644))
	cmd.SetArgs(append(args, tmplPath))
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return cmd.ExecuteContext(t.Context())
}

func TestTemplateValidateParamAndSet(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"without a value", nil, "field `mounts[0].location` must be an absolute path"},
		{"--param", []string{"--param", "Dir=" + dir}, ""},
		{"--set", []string{"--set", fmt.Sprintf(".param.Dir = %q", dir)}, ""},
		{"undefined --param", []string{"--param", "Missing=" + dir}, "template does not define param `Missing`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runTemplateCommand(t, newTemplateValidateCommand(), tt.args...)
			if tt.wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestTemplateCopyParamAndSet(t *testing.T) {
	cmd := newTemplateCopyCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	assert.NilError(t, runTemplateCommand(t, cmd, "--param", "Dir=/mnt", "--set", ".cpus = 3"))

	var y limatype.LimaYAML
	assert.NilError(t, limayaml.Unmarshal(out.Bytes(), &y, "copy"))
	assert.Equal(t, y.Param["Dir"], "/mnt")
	assert.Equal(t, *y.CPUs, 3)
}
