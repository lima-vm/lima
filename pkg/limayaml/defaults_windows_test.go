//go:build windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestFillDefaultWSL2RejectedMount(t *testing.T) {
	for _, tc := range []struct {
		name string
		loc  string
	}{
		{"UNC path", `\\server\share\probe`},
		{"relative path", `.\relative\path`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := limatype.LimaYAML{
				VMType: new(limatype.WSL2),
				Mounts: []limatype.Mount{{Location: tc.loc}},
			}
			FillDefault(t.Context(), &y, &limatype.LimaYAML{}, &limatype.LimaYAML{}, filepath.Join(t.TempDir(), "lima.yaml"), false)
			assert.Assert(t, y.Mounts[0].MountPoint == nil)
			err := Validate(&y, false)
			assert.ErrorContains(t, err, "field `mounts[0].mountPoint` must be specified explicitly")
		})
	}
}
