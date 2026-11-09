//go:build darwin && cgo

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/usbip"
)

const uartBusid = "1-0x01100000"

var uart = usbip.DeviceInfo{Vendor: 0x403, Product: 0x6015, ProductName: "UART", BusID: uartBusid}

func newUSBTestInstance(t *testing.T, name string, devices ...limatype.USBDevice) *limatype.Instance {
	t.Helper()
	home, err := filepath.EvalSymlinks(os.Getenv("LIMA_HOME"))
	assert.NilError(t, err)
	inst := &limatype.Instance{Name: name, Dir: filepath.Join(home, name), Config: &limatype.LimaYAML{
		GuestInstallPrefix: new("/usr/local"),
		USB:                limatype.USB{Devices: devices},
	}}
	assert.NilError(t, os.Mkdir(inst.Dir, 0o755))
	return inst
}

// holdDevice flocks the lock file as a hostagent does.
func holdDevice(t *testing.T, busid, holder string) (lockPath string, release func()) {
	t.Helper()
	lockPath = filepath.Join(os.Getenv("LIMA_HOME"), "_usbip-"+busid+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o600)
	assert.NilError(t, err)
	assert.NilError(t, unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB))
	_, err = f.WriteString(holder)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return lockPath, func() { _ = f.Close() }
}

func TestUSBListShowsHolderWithoutGuest(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newUSBTestInstance(t, "vm1")
	grant := usbip.EntryFor(uart)
	grant.Name = "serial"
	assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{grant}))
	rows := func(args ...string) []string {
		var out bytes.Buffer
		assert.NilError(t, writeUSBList(&out, []usbip.DeviceInfo{uart}, args, false))
		return strings.Fields(strings.Split(out.String(), "\n")[1])
	}

	assert.Equal(t, rows()[1], "0403:6015")
	assert.Equal(t, rows()[5], "-", "free device, no instance named")
	assert.Equal(t, rows()[4], "-")
	assert.Equal(t, rows("vm1")[4], "serial", "named by the instance asked about")
	assert.Equal(t, rows("vm1")[5], "-")

	holdDevice(t, uartBusid, "vm1")
	assert.Equal(t, rows("vm2")[5], "vm1", "holder shows whichever instance is asked about")
}

func TestUSBListJSON(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newUSBTestInstance(t, "vm1")
	grant := usbip.EntryFor(uart)
	grant.Name = "serial"
	assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{grant}))
	holdDevice(t, uartBusid, "vm1")
	free := usbip.DeviceInfo{Vendor: 0x1111, Product: 0x2222, BusID: "2-0x02100000", VendorName: "ACME"}

	var out bytes.Buffer
	assert.NilError(t, writeUSBList(&out, []usbip.DeviceInfo{uart, free}, nil, true))
	assert.Equal(t, out.String(), `{"busid":"1-0x01100000","vendorID":"0403","productID":"6015","vendor":"","product":"UART","name":"serial","attachedTo":"vm1"}
{"busid":"2-0x02100000","vendorID":"1111","productID":"2222","vendor":"ACME","product":"","name":"","attachedTo":""}
`)
}

func TestUSBCommandsRunRootHookAndWarnOnce(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	hook := logrustest.NewGlobal()
	ran := 0
	root := &cobra.Command{
		Use:           "limactl",
		SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			ran++
			return nil
		},
	}
	root.AddGroup(&cobra.Group{ID: advancedCommand, Title: advancedCommand})
	root.AddCommand(newUsbCommand())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"usb", "list", "--json"})
	assert.NilError(t, root.Execute())

	assert.Equal(t, ran, 1)
	warns := 0
	for _, e := range hook.AllEntries() {
		if e.Message == "`limactl usb` is experimental" {
			warns++
		}
	}
	assert.Equal(t, warns, 1)
	assert.Assert(t, !strings.Contains(out.String(), "BUSID"), "--json printed a table")
}

// No PID files, so store.Inspect reports the instance as stopped.
func newStoppedInstance(t *testing.T, config string) *limatype.Instance {
	t.Helper()
	inst := newUSBTestInstance(t, "vm1")
	yaml := config + "\nimages: [{location: /nonexistent.img}]\n"
	assert.NilError(t, os.WriteFile(filepath.Join(inst.Dir, "lima.yaml"), []byte(yaml), 0o644))
	return inst
}

func TestInspectUsesRealName(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newStoppedInstance(t, "vmType: vz\nusb: {enabled: true}")
	if _, err := os.Stat(filepath.Join(filepath.Dir(inst.Dir), "VM1")); err != nil {
		t.Skip("case-sensitive volume")
	}
	got, err := inspectVZInstance(t.Context(), "VM1")
	assert.NilError(t, err)
	assert.Equal(t, got.Name, "vm1")
}
