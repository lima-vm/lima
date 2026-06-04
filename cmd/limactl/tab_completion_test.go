// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func TestSetFlagCompletion(t *testing.T) {
	tests := []struct {
		name       string
		toComplete string
		wantMatch  []string // We check that at least these values are present in the output
		wantNil    bool
	}{
		{
			name:       "Empty input returns properties",
			toComplete: "",
			wantMatch:  []string{".vmType=", ".arch="},
		},
		{
			name:       "Partial property filter",
			toComplete: ".vm",
			wantMatch:  []string{".vmType=", ".vmOpts="},
		},
		{
			name:       "Exact property with equals returns all enum values",
			toComplete: ".vmType=",
			wantMatch:  []string{".vmType=qemu", ".vmType=vz", ".vmType=wsl2"},
		},
		{
			name:       "Nested property filter",
			toComplete: ".ssh.",
			wantMatch:  []string{".ssh.localPort=", ".ssh.forwardAgent="},
		},
		{
			name:       "Partial value filter",
			toComplete: ".vmType=q",
			wantMatch:  []string{".vmType=qemu"},
		},
		{
			name:       "Unknown property after equals returns nil",
			toComplete: ".unknown=",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := setFlagCompletion(&cobra.Command{}, nil, tt.toComplete)

			if tt.wantNil {
				assert.Assert(t, got == nil, "expected nil completions")
				return
			}

			for _, want := range tt.wantMatch {
				assert.Assert(t, slices.Contains(got, want), "expected to find %q in completions, but didn't. Got: %v", want, got)
			}
		})
	}
}
