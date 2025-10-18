// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

const separator = string(filepath.Separator)

var (
	vmtype = limatype.QEMU
	goarch = limatype.NewArch(runtime.GOARCH)
	space  = strings.Repeat(" ", len(goarch)-4)
)

var instance = limatype.Instance{
	Name:       "foo",
	Status:     limatype.StatusStopped,
	VMType:     vmtype,
	Arch:       goarch,
	Dir:        "dir",
	SSHAddress: "127.0.0.1",
}

var table = "NAME    STATUS     SSH            CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    0       0B        0B      dir\n"

var tableEmu = "NAME    STATUS     SSH            ARCH       CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    unknown    0       0B        0B      dir\n"

var tableHome = "NAME    STATUS     SSH            CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    0       0B        0B      ~" + separator + "dir\n"

var tableAll = "NAME    STATUS     SSH            VMTYPE    ARCH" + space + "    CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    " + vmtype + "      " + goarch + "    0       0B        0B      dir\n"

// for width 60, everything is hidden
var table60 = "NAME    STATUS     CPUS    MEMORY    DISK\n" +
	"foo     Stopped    0       0B        0B\n"

// for width 60, identical is hidden (type/arch)
var table60i = "NAME    STATUS     CPUS    MEMORY    DISK\n" +
	"foo     Stopped    0       0B        0B\n"

// for width 60, different arch is still shown
var table60d = "NAME    STATUS     ARCH       CPUS    MEMORY    DISK\n" +
	"foo     Stopped    unknown    0       0B        0B\n"

// for width 80, vmtype is hidden
var table80 = "NAME    STATUS     CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    0       0B        0B      dir\n"

// for width 100, ssh is hidden
var table100 = "NAME    STATUS     VMTYPE    ARCH" + space + "    CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    " + vmtype + "      " + goarch + "    0       0B        0B      dir\n"

// for width 100, different ssh is still shown
var table100d = "NAME    STATUS     SSH          ARCH" + space + "    CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    1.2.3.4:0    " + goarch + "    0       0B        0B      dir\n"

// for width 120, nothing is hidden
var table120 = "NAME    STATUS     SSH            VMTYPE    ARCH" + space + "    CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    " + vmtype + "      " + goarch + "    0       0B        0B      dir\n"

// for width 60, directory is hidden (if not identical)
var tableTwo = "NAME    STATUS     VMTYPE    ARCH       CPUS    MEMORY    DISK\n" +
	"foo     Stopped    qemu      x86_64     0       0B        0B\n" +
	"bar     Stopped    vz        aarch64    0       0B        0B\n"

func TestPrintInstanceTable(t *testing.T) {
	var buf bytes.Buffer
	instances := []*limatype.Instance{&instance}
	err := PrintInstances(&buf, instances, "table", nil)
	assert.NilError(t, err)
	assert.Equal(t, table, buf.String())
}

func TestPrintInstanceTableEmu(t *testing.T) {
	var buf bytes.Buffer
	instance1 := instance
	instance1.Arch = "unknown"
	instances := []*limatype.Instance{&instance1}
	err := PrintInstances(&buf, instances, "table", nil)
	assert.NilError(t, err)
	assert.Equal(t, tableEmu, buf.String())
}

func TestPrintInstanceTableHome(t *testing.T) {
	var buf bytes.Buffer
	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)
	instance1 := instance
	instance1.Dir = filepath.Join(homeDir, "dir")
	instances := []*limatype.Instance{&instance1}
	err = PrintInstances(&buf, instances, "table", nil)
	assert.NilError(t, err)
	assert.Equal(t, tableHome, buf.String())
}

func TestPrintInstanceTable60(t *testing.T) {
	var buf bytes.Buffer
	instances := []*limatype.Instance{&instance}
	options := PrintOptions{TerminalWidth: 60}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, table60, buf.String())
}

func TestPrintInstanceTable60SameArch(t *testing.T) {
	var buf bytes.Buffer
	instances := []*limatype.Instance{&instance}
	options := PrintOptions{TerminalWidth: 60}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, table60i, buf.String())
}

func TestPrintInstanceTable60DiffArch(t *testing.T) {
	var buf bytes.Buffer
	instance1 := instance
	instance1.Arch = limatype.NewArch("unknown")
	instances := []*limatype.Instance{&instance1}
	options := PrintOptions{TerminalWidth: 60}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, table60d, buf.String())
}

func TestPrintInstanceTable80(t *testing.T) {
	var buf bytes.Buffer
	instances := []*limatype.Instance{&instance}
	options := PrintOptions{TerminalWidth: 80}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, table80, buf.String())
}

func TestPrintInstanceTable100Localhost(t *testing.T) {
	var buf bytes.Buffer
	instances := []*limatype.Instance{&instance}
	options := PrintOptions{TerminalWidth: 100}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, table100, buf.String())
}

func TestPrintInstanceTable100IPAddress(t *testing.T) {
	var buf bytes.Buffer
	instance1 := instance
	instance1.SSHAddress = "1.2.3.4"
	instances := []*limatype.Instance{&instance1}
	options := PrintOptions{TerminalWidth: 100}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, table100d, buf.String())
}

func TestPrintInstanceTable120(t *testing.T) {
	var buf bytes.Buffer
	instances := []*limatype.Instance{&instance}
	options := PrintOptions{TerminalWidth: 120}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, table120, buf.String())
}

func TestPrintInstanceTableAll(t *testing.T) {
	var buf bytes.Buffer
	instances := []*limatype.Instance{&instance}
	options := PrintOptions{TerminalWidth: 40, AllFields: true}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, tableAll, buf.String())
}

func TestPrintInstanceTableTwo(t *testing.T) {
	var buf bytes.Buffer
	instance1 := instance
	instance1.Name = "foo"
	instance1.VMType = limatype.QEMU
	instance1.Arch = limatype.X8664
	instance2 := instance
	instance2.Name = "bar"
	instance2.VMType = limatype.VZ
	instance2.Arch = limatype.AARCH64
	instances := []*limatype.Instance{&instance1, &instance2}
	options := PrintOptions{TerminalWidth: 60}
	err := PrintInstances(&buf, instances, "table", &options)
	assert.NilError(t, err)
	assert.Equal(t, tableTwo, buf.String())
}
