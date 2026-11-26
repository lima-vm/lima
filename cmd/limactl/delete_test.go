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

func TestDeleteActionBatchErrorAccumulation(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)

	// Create an instance directory with invalid lima.yaml so store.Inspect fails
	inst1Dir := filepath.Join(limaHome, "inst1")
	assert.NilError(t, os.MkdirAll(inst1Dir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(inst1Dir, filenames.LimaYAML), []byte("invalid: yaml: ["), 0o644))

	// Second instance directory also with invalid lima.yaml
	inst2Dir := filepath.Join(limaHome, "inst2")
	assert.NilError(t, os.MkdirAll(inst2Dir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(inst2Dir, filenames.LimaYAML), []byte("invalid: yaml: ["), 0o644))

	cmd := newTestDeleteCommand(t)

	err := deleteAction(cmd, []string{"inst1", "inst2"})
	assert.Assert(t, err != nil, "deleteAction should return accumulated errors")
	assert.ErrorContains(t, err, "inst1")
	assert.ErrorContains(t, err, "inst2")
}

func newTestDeleteCommand(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().BoolP("force", "f", false, "")
	cmd.SetContext(t.Context())
	return cmd
}

func writeTestInstance(t *testing.T, limaHome, name, yamlContent string, protected bool) string {
	t.Helper()
	instDir := filepath.Join(limaHome, name)
	assert.NilError(t, os.MkdirAll(instDir, 0o755))
	assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.LimaYAML), []byte(yamlContent), 0o644))
	if protected {
		assert.NilError(t, os.WriteFile(filepath.Join(instDir, filenames.Protected), nil, 0o644))
	}
	return instDir
}

func TestDeleteActionBatchMixedSuccessAndFailure(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)

	const validYAML = "images: [{location: /}]\ncpus: 2\n"
	prot1Dir := writeTestInstance(t, limaHome, "prot1", validYAML, true)
	ok1Dir := writeTestInstance(t, limaHome, "ok1", validYAML, false)
	invalid1Dir := writeTestInstance(t, limaHome, "invalid1", "invalid: yaml: [", false)
	ok2Dir := writeTestInstance(t, limaHome, "ok2", validYAML, false)

	err := deleteAction(newTestDeleteCommand(t), []string{"prot1", "ok1", "invalid1", "ok2", "nonexistent"})
	assert.Assert(t, err != nil, "deleteAction should return accumulated errors")
	assert.ErrorContains(t, err, "prot1")
	assert.ErrorContains(t, err, "invalid1")

	for _, removed := range []string{ok1Dir, ok2Dir} {
		_, statErr := os.Stat(removed)
		assert.Assert(t, os.IsNotExist(statErr), "%s should have been deleted", removed)
	}
	for _, kept := range []string{prot1Dir, invalid1Dir} {
		_, statErr := os.Stat(kept)
		assert.NilError(t, statErr, "%s should still exist after failed delete", kept)
	}
}

func TestDeleteActionSuccessAndSingleFailure(t *testing.T) {
	limaHome := t.TempDir()
	t.Setenv("LIMA_HOME", limaHome)

	const validYAML = "images: [{location: /}]\ncpus: 2\n"
	ok1Dir := writeTestInstance(t, limaHome, "ok1", validYAML, false)
	ok2Dir := writeTestInstance(t, limaHome, "ok2", validYAML, false)
	protDir := writeTestInstance(t, limaHome, "prot", validYAML, true)

	assert.NilError(t, deleteAction(newTestDeleteCommand(t), []string{"ok1", "ok2"}))
	for _, removed := range []string{ok1Dir, ok2Dir} {
		_, statErr := os.Stat(removed)
		assert.Assert(t, os.IsNotExist(statErr), "%s should have been deleted", removed)
	}

	err := deleteAction(newTestDeleteCommand(t), []string{"prot"})
	assert.ErrorContains(t, err, "prot")
	_, statErr := os.Stat(protDir)
	assert.NilError(t, statErr, "protected instance directory should still exist")
}
