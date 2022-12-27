package store

import (
	"bytes"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lima-vm/lima/pkg/limayaml"
	"gotest.tools/v3/assert"
)

const separator = string(filepath.Separator)

var vmtype = limayaml.QEMU
var goarch = limayaml.NewArch(runtime.GOARCH)

var instance = Instance{
	Name:   "foo",
	Status: StatusStopped,
	VMType: vmtype,
	Arch:   goarch,
	Dir:    "dir",
}

var table = "NAME    STATUS     SSH            CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    0       0B        0B      dir\n"

var tableEmu = "NAME    STATUS     SSH            ARCH       CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    unknown    0       0B        0B      dir\n"

var tableHome = "NAME    STATUS     SSH            CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    0       0B        0B      ~" + separator + "dir\n"

var tableAll = "NAME    STATUS     SSH            VMTYPE    ARCH      CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    " + vmtype + "      " + goarch + "    0       0B        0B      dir\n"

var tableTwo = "NAME    STATUS     SSH            VMTYPE    ARCH       CPUS    MEMORY    DISK    DIR\n" +
	"foo     Stopped    127.0.0.1:0    qemu      x86_64     0       0B        0B      dir\n" +
	"bar     Stopped    127.0.0.1:0    vz        aarch64    0       0B        0B      dir\n"

func TestPrintInstanceTable(t *testing.T) {
	var buf bytes.Buffer
	instances := []*Instance{&instance}
	PrintInstances(&buf, instances, "table", nil)
	assert.Equal(t, table, buf.String())
}

func TestPrintInstanceTableEmu(t *testing.T) {
	var buf bytes.Buffer
	instance1 := instance
	instance1.Arch = "unknown"
	instances := []*Instance{&instance1}
	PrintInstances(&buf, instances, "table", nil)
	assert.Equal(t, tableEmu, buf.String())
}

func TestPrintInstanceTableHome(t *testing.T) {
	var buf bytes.Buffer
	u, err := user.Current()
	assert.NilError(t, err)
	instance1 := instance
	instance1.Dir = filepath.Join(u.HomeDir, "dir")
	instances := []*Instance{&instance1}
	PrintInstances(&buf, instances, "table", nil)
	assert.Equal(t, tableHome, buf.String())
}

func TestPrintInstanceTableAll(t *testing.T) {
	var buf bytes.Buffer
	instances := []*Instance{&instance}
	options := PrintOptions{AllFields: true}
	PrintInstances(&buf, instances, "table", &options)
	assert.Equal(t, tableAll, buf.String())
}

func TestPrintInstanceTableTwo(t *testing.T) {
	var buf bytes.Buffer
	instance1 := instance
	instance1.Name = "foo"
	instance1.VMType = limayaml.QEMU
	instance1.Arch = limayaml.X8664
	instance2 := instance
	instance2.Name = "bar"
	instance2.VMType = limayaml.VZ
	instance2.Arch = limayaml.AARCH64
	instances := []*Instance{&instance1, &instance2}
	options := PrintOptions{}
	PrintInstances(&buf, instances, "table", &options)
	assert.Equal(t, tableTwo, buf.String())
}
