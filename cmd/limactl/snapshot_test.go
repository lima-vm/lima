// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestPrintSnapshots(t *testing.T) {
	createdAt := time.Date(2026, time.September, 16, 20, 50, 52, 0, time.UTC)
	snapshots := []driver.Snapshot{
		{ID: "1", Tag: "initial", CreatedAt: &createdAt},
		{ID: "2", Tag: "before package install"},
	}

	tests := []struct {
		name       string
		quiet      bool
		jsonFormat bool
		want       string
	}{
		{
			name: "table",
			want: "ID    TAG                       CREATED\n" +
				"1     initial                   2026-09-16T20:50:52Z\n" +
				"2     before package install    -\n",
		},
		{
			name:  "quiet",
			quiet: true,
			want:  "initial\nbefore package install\n",
		},
		{
			name:       "json",
			jsonFormat: true,
			want: "{\"id\":\"1\",\"tag\":\"initial\",\"createdAt\":\"2026-09-16T20:50:52Z\"}\n" +
				"{\"id\":\"2\",\"tag\":\"before package install\"}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bytes.Buffer
			assert.NilError(t, printSnapshots(&got, snapshots, tt.quiet, tt.jsonFormat))
			assert.Equal(t, got.String(), tt.want)
		})
	}
}

func TestFindSnapshotIDByTag(t *testing.T) {
	snapshots := []driver.Snapshot{
		{ID: "1", Tag: "initial"},
		{ID: "2", Tag: "duplicate"},
		{ID: "3", Tag: "duplicate"},
	}

	id, err := findSnapshotIDByTag(snapshots, "initial")
	assert.NilError(t, err)
	assert.Equal(t, id, "1")

	_, err = findSnapshotIDByTag(snapshots, "duplicate")
	assert.ErrorContains(t, err, `snapshot tag "duplicate" is not unique`)

	_, err = findSnapshotIDByTag(snapshots, "missing")
	assert.ErrorContains(t, err, `snapshot tag "missing" not found`)
}

func TestSnapshotActions_InstanceErrors(t *testing.T) {
	limaDir := t.TempDir()
	t.Setenv("LIMA_HOME", limaDir)
	instanceDir := filepath.Join(limaDir, "broken")
	assert.NilError(t, os.MkdirAll(instanceDir, 0o700))
	assert.NilError(t, os.WriteFile(filepath.Join(instanceDir, filenames.LimaYAML), []byte("cpus: [\n"), 0o600))

	createCmd := newSnapshotCreateCommand()
	createCmd.SetArgs([]string{"broken", "--tag", "snap1"})

	delCmd := newSnapshotDeleteCommand()
	delCmd.SetArgs([]string{"broken", "--tag", "snap1"})

	applyCmd := newSnapshotApplyCommand()
	applyCmd.SetArgs([]string{"broken", "--tag", "snap1"})

	listCmd := newSnapshotListCommand()
	listCmd.SetArgs([]string{"broken"})

	for _, cmd := range []*cobra.Command{createCmd, delCmd, applyCmd, listCmd} {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
	}

	assert.ErrorContains(t, createCmd.Execute(), "errors inspecting instance")
	assert.ErrorContains(t, delCmd.Execute(), "errors inspecting instance")
	assert.ErrorContains(t, applyCmd.Execute(), "errors inspecting instance")
	assert.ErrorContains(t, listCmd.Execute(), "errors inspecting instance")
}
