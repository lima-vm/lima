// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/driver"
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
