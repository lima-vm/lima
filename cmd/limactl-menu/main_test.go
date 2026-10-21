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
	assert.Assert(t, strings.Contains(formatted, "4GiB"), formatted)
	assert.Assert(t, !strings.Contains(formatted, "CPU"), formatted)
}

func TestFmtInstHasSeparateCPUsAndMemoryCells(t *testing.T) {
	inst := &limatype.Instance{
		Name: "default", Status: limatype.StatusRunning, Arch: limatype.AARCH64,
		CPUs: 4, Memory: 4 << 30, VMType: limatype.VZ,
	}
	fields := strings.Fields(fmtInst(inst))
	assert.DeepEqual(t, fields[1:], []string{"default", "Running", "aarch64", "4", "4GiB", "vz"})
}

func TestFmtHeaderTitles(t *testing.T) {
	assert.DeepEqual(t, strings.Fields(fmtHeader()),
		[]string{"NAME", "STATUS", "ARCH", "CPUS", "MEMORY", "VMTYPE"})
}

func TestFmtHeaderIsAlignedWithRow(t *testing.T) {
	inst := &limatype.Instance{
		Name: "default", Status: limatype.StatusRunning, Arch: limatype.AARCH64,
		CPUs: 4, Memory: 4 << 30, VMType: limatype.VZ,
	}
	_, row, found := strings.Cut(fmtInst(inst), "  ")
	assert.Assert(t, found, fmtInst(inst))
	header := strings.TrimPrefix(fmtHeader(), strings.Repeat(" ", statusIconWidth))
	assert.DeepEqual(t, fieldStarts(header), fieldStarts(row))
}

func fieldStarts(s string) []int {
	var starts []int
	inField := false
	for i, r := range s {
		switch {
		case r == ' ':
			inField = false
		case !inField:
			starts = append(starts, i)
			inField = true
		}
	}
	return starts
}

func TestFmtInstEscapesProtectedMarker(t *testing.T) {
	inst := &limatype.Instance{
		Name: "default", Status: limatype.StatusRunning, CPUs: 4, Memory: 4 << 30,
		VMType: limatype.VZ, Protected: true,
	}
	assert.Assert(t, strings.HasSuffix(fmtInst(inst), " [PROTECTED[]"), fmtInst(inst))
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
