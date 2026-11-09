//go:build darwin && cgo

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// endOnRevoke releases the lock once limactl empties it, as a session's Gone poll does.
func endOnRevoke(lockPath string, release func()) {
	go func() {
		for {
			if fi, err := os.Stat(lockPath); err == nil && fi.Size() == 0 {
				release()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
}

func stubGuest(t *testing.T, fn func() (string, error)) (calls *[]string) {
	t.Helper()
	old := guestCommand
	t.Cleanup(func() { guestCommand = old })
	calls = &[]string{}
	guestCommand = func(_ context.Context, _ *limatype.Instance, command string) (string, error) {
		*calls = append(*calls, command)
		return fn()
	}
	return calls
}

func TestAttachHeldDeviceFailsBeforeGuest(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newUSBTestInstance(t, "vm1")
	calls := stubGuest(t, func() (string, error) { return "", nil })

	_, release := holdDevice(t, uartBusid, "vm1")
	err := attachDevice(t.Context(), inst, uart, "")
	assert.ErrorContains(t, err, `is already attached to instance "vm1"`)
	assert.Assert(t, !strings.Contains(err.Error(), "detach it there"), err.Error())
	release()

	holdDevice(t, uartBusid, "vm2")
	assert.ErrorContains(t, attachDevice(t.Context(), inst, uart, ""), `attached to instance "vm2"; detach it there first`)

	assert.Equal(t, len(*calls), 0)
	list, err := usbip.ReadAllowlist(inst.Dir)
	assert.NilError(t, err)
	assert.Equal(t, len(list), 0, "refused attach wrote a grant")
}

func TestAttachWritesGrantBeforeGuest(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newUSBTestInstance(t, "vm1")
	calls := stubGuest(t, func() (string, error) {
		list, err := usbip.ReadAllowlist(inst.Dir)
		assert.NilError(t, err)
		assert.Equal(t, len(list), 1, "guest ran before the grant")
		return "", nil
	})
	assert.NilError(t, attachDevice(t.Context(), inst, uart, "serial"))
	assert.DeepEqual(t, *calls, []string{"sudo /usr/local/bin/lima-guestagent usbip attach --busid " + uartBusid})
	list, err := usbip.ReadAllowlist(inst.Dir)
	assert.NilError(t, err)
	assert.Equal(t, list[0].Name, "serial")
	assert.Equal(t, list[0].Busid, uartBusid)
}

func TestAttachRollsBack(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newUSBTestInstance(t, "vm1")
	stubGuest(t, func() (string, error) { return "\x1b[31mhost refused\x1b[0m\n", errors.New("exit 1") })
	for _, seed := range [][]usbip.AllowEntry{
		{{VendorID: "1111", ProductID: "2222"}},
		{{VendorID: "0403", ProductID: "6015"}, {VendorID: "1111", ProductID: "2222"}},
		{{Name: "uart", VendorID: "0403", ProductID: "6015", BusAddr: uartBusid}},
		{{VendorID: "0403", ProductID: "6015"}, {VendorID: "0403", ProductID: "6015", BusAddr: uartBusid, Denied: true}},
	} {
		for _, name := range []string{"", "x"} {
			assert.NilError(t, usbip.WriteAllowlist(inst.Dir, seed))
			err := attachDevice(t.Context(), inst, uart, name)
			assert.ErrorContains(t, err, "host refused")
			assert.Assert(t, !strings.Contains(err.Error(), "\x1b"), "control chars reached the terminal")
			list, err := usbip.ReadAllowlist(inst.Dir)
			assert.NilError(t, err)
			assert.DeepEqual(t, list, seed)
		}
	}

	// SSH can die after the import worked, which leaves a session to end.
	stubGuest(t, func() (string, error) {
		endOnRevoke(holdDevice(t, uartBusid, "vm1"))
		return "", errors.New("ssh died")
	})
	assert.ErrorContains(t, attachDevice(t.Context(), inst, uart, ""), "ssh died")
	_, held := usbip.Holder(uartBusid)
	assert.Assert(t, !held, "session outlived the rollback")
}

func TestAttachKeepsYAMLBusAddr(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newUSBTestInstance(t, "vm1")
	seed := []usbip.AllowEntry{{Name: "uart", VendorID: "0403", ProductID: "6015", BusAddr: uartBusid}}
	stubGuest(t, func() (string, error) { return "", nil })

	for _, name := range []string{"", "x"} {
		assert.NilError(t, usbip.WriteAllowlist(inst.Dir, seed))
		assert.NilError(t, attachDevice(t.Context(), inst, uart, name))
		usbip.NewProvider(inst.Dir).Unplugged(uartBusid)
		list, err := usbip.ReadAllowlist(inst.Dir)
		assert.NilError(t, err)
		assert.DeepEqual(t, list, seed)
	}
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

func TestBusidCompletions(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	free := usbip.DeviceInfo{Vendor: 0x1111, Product: 0x2222, BusID: "2-0x02100000"}
	holdDevice(t, uartBusid, "vm1")
	hosts := []usbip.DeviceInfo{uart, free}

	assert.DeepEqual(t, busidCompletions(hosts, ""), []cobra.Completion{uartBusid + "\tUART", free.BusID + "\t1111:2222"})
	assert.DeepEqual(t, busidCompletions(hosts, "vm1"), []cobra.Completion{uartBusid + "\tUART"})
	assert.Equal(t, len(busidCompletions(hosts, "vm2")), 0)
}

func TestBoundedOutput(t *testing.T) {
	old := guestTimeout
	t.Cleanup(func() { guestTimeout = old })
	guestTimeout = 500 * time.Millisecond

	t.Run("output is capped", func(t *testing.T) {
		out, err := boundedOutput(t.Context(), "sh", "-c", fmt.Sprintf("head -c %d /dev/zero; echo x >&2", 4*guestOutputLimit))
		assert.NilError(t, err)
		assert.Assert(t, len(out) == guestOutputLimit, "got %d", len(out))
	})
	t.Run("exit status survives", func(t *testing.T) {
		out, err := boundedOutput(t.Context(), "sh", "-c", "echo refused >&2; exit 3")
		assert.ErrorContains(t, err, "exit status 3")
		assert.Equal(t, out, "refused\n")
	})
	t.Run("hung guest times out", func(t *testing.T) {
		_, err := boundedOutput(t.Context(), "sleep", "30")
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
	t.Run("background child holding the pipes is success", func(t *testing.T) {
		guestTimeout = 10 * time.Second
		start := time.Now()
		_, err := boundedOutput(t.Context(), "sh", "-c", "sleep 30 &")
		assert.NilError(t, err)
		assert.Assert(t, time.Since(start) < 5*time.Second, "waited %s", time.Since(start))
	})
}

// No PID files, so store.Inspect reports the instance as stopped.
func newStoppedInstance(t *testing.T, config string) *limatype.Instance {
	t.Helper()
	inst := newUSBTestInstance(t, "vm1")
	yaml := config + "\nimages: [{location: /nonexistent.img}]\n"
	assert.NilError(t, os.WriteFile(filepath.Join(inst.Dir, "lima.yaml"), []byte(yaml), 0o644))
	return inst
}

func TestRequireVZUSBInstanceGates(t *testing.T) {
	for config, want := range map[string]string{
		"vmType: vz\nusb: {enabled: true}":                                                        "is not running",
		"vmType: vz\nusb: {devices: [{vendorID: \"0403\", productID: \"6015\"}]}":                 "is not running",
		"vmType: vz\nusb: {enabled: false}":                                                       "does not have USB enabled",
		"vmType: vz\nusb: {enabled: false, devices: [{vendorID: \"0403\", productID: \"6015\"}]}": "does not have USB enabled",
		"vmType: qemu\nusb: {enabled: true}":                                                      "is not a vz instance",
		"vmType: vz\nusb: {enabled: true}\nplain: true\nmountType: reverse-sshfs":                 "plain mode",
		"vmType: vz\nos: Darwin\nusb: {enabled: true}":                                            "not a Linux guest",
	} {
		t.Run(want, func(t *testing.T) {
			t.Setenv("LIMA_HOME", t.TempDir())
			newStoppedInstance(t, config)
			_, err := requireVZUSBInstance(t.Context(), "vm1")
			assert.ErrorContains(t, err, want)
		})
	}
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

func TestBrokenConfigIsAnError(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newUSBTestInstance(t, "vm1")
	assert.NilError(t, os.WriteFile(filepath.Join(inst.Dir, "lima.yaml"), []byte("cpus: [\n"), 0o644))
	_, err := requireVZUSBInstance(t.Context(), "vm1")
	assert.ErrorContains(t, err, "invalid config")
	_, err = inspectVZInstance(t.Context(), "vm1")
	assert.ErrorContains(t, err, "invalid config")
}
