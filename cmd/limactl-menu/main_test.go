// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestFmtInstUsesBinaryMemoryUnits(t *testing.T) {
	inst := &limatype.Instance{
		Name: "default", Status: limatype.StatusRunning, CPUs: 4, Memory: 4 << 30,
	}
	formatted := fmtInst(inst)
	assert.Assert(t, strings.Contains(formatted, "4CPU/4GiB"), formatted)
}

func TestFmtInstShowsVMType(t *testing.T) {
	inst := &limatype.Instance{
		Name: "default", Status: limatype.StatusRunning, CPUs: 4, Memory: 4 << 30,
		VMType: limatype.VZ, SSHAddress: "127.0.0.1", SSHLocalPort: 60022,
	}
	formatted := fmtInst(inst)
	assert.Assert(t, strings.Contains(formatted, limatype.VZ), formatted)
	assert.Assert(t, !strings.Contains(formatted, "60022"), formatted)

	inst.VMType = ""
	assert.Assert(t, strings.HasSuffix(fmtInst(inst), "  -"), fmtInst(inst))
}
