// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
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
	_, row, found := strings.Cut(fmtInst(inst), "[white]  ")
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

func TestGenDocContentModeCLI(t *testing.T) {
	dir := t.TempDir()
	app := newApp()
	app.SetArgs([]string{"generate-doc", "--content-mode=cli", dir})
	assert.NilError(t, app.Execute())

	b, err := os.ReadFile(filepath.Join(dir, "limactl_menu.md"))
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(b), "title: menu\n"), string(b))
	assert.Assert(t, strings.Contains(string(b), "limactl menu"), string(b))
	_, err = os.Stat(filepath.Join(dir, "limactl_menu_generate-doc.md"))
	assert.Assert(t, os.IsNotExist(err), "hidden command should not be documented")
}
