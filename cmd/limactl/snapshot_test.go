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
		{ID: "1", Name: "initial", CreatedAt: &createdAt},
		{ID: "2", Name: "before package install"},
	}

	tests := []struct {
		name  string
		quiet bool
		want  string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bytes.Buffer
			assert.NilError(t, printSnapshots(&got, snapshots, tt.quiet))
			assert.Equal(t, got.String(), tt.want)
		})
	}
}
