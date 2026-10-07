// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krunkit

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestValidateConfigRejectsBlockDevices(t *testing.T) {
	err := validateConfig(&limatype.LimaYAML{BlockDevices: []string{"/dev/disk4"}})
	assert.ErrorContains(t, err, "field `blockDevices` is not supported for vmType: krunkit")
}

func TestValidateRejectsHostLoopbackFalse(t *testing.T) {
	// krunkit's netstack does not honor the option, so the refusal is all that stops a silent no-op.
	l := &LimaKrunkitDriver{Instance: &limatype.Instance{Config: &limatype.LimaYAML{NetworkOpts: limatype.NetworkOpts{HostLoopback: new(false)}}}}
	assert.ErrorContains(t, l.Validate(t.Context()), "`networkOpts.hostLoopback: false` is not supported in krunkit driver")
}

func TestValidateSnapshotTag(t *testing.T) {
	tests := []struct {
		tag     string
		wantErr bool
	}{
		{tag: "base"},
		{tag: "snap-1"},
		{tag: "", wantErr: true},
		{tag: ".", wantErr: true},
		{tag: "..", wantErr: true},
		{tag: "../evil", wantErr: true},
		{tag: "a/b", wantErr: true},
		{tag: "/abs", wantErr: true},
		{tag: `a\b`, wantErr: true},
		{tag: "a b", wantErr: true},
		{tag: "a\tb", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.tag), func(t *testing.T) {
			err := validateSnapshotTag(tt.tag)
			if tt.wantErr {
				assert.ErrorContains(t, err, "invalid snapshot tag")
				return
			}
			assert.NilError(t, err)
		})
	}
}
