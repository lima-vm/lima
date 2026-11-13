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

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/usbip"
)

const uartBusid = "1-0x01100000"

var uart = usbip.DeviceInfo{Vendor: 0x403, Product: 0x6015, ProductName: "UART", Busid: uartBusid}

func newUSBTestInstance(t *testing.T, name string, devices ...limatype.USBDevice) *limatype.Instance {
	t.Helper()
	home, err := filepath.EvalSymlinks(os.Getenv("LIMA_HOME"))
	assert.NilError(t, err)
	inst := &limatype.Instance{Name: name, Dir: filepath.Join(home, name), Config: &limatype.LimaYAML{
		GuestInstallPrefix: new("/usr/local"),
		VMOpts:             limatype.VMOpts{limatype.VZ: limatype.VZOpts{USB: limatype.USB{Devices: devices}}},
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

func TestDetach(t *testing.T) {
	grant := usbip.EntryFor(uart)
	hook := logrustest.NewGlobal()
	run := func(t *testing.T, inst *limatype.Instance, hosts []usbip.DeviceInfo, args ...string) string {
		t.Helper()
		args = append(args, "", "", "")
		hook.Reset()
		assert.NilError(t, detachDevice(inst, hosts, args[0], args[1], args[2]))
		var msgs []string
		for _, e := range hook.AllEntries() {
			msgs = append(msgs, e.Message)
		}
		return strings.Join(msgs, "\n")
	}

	t.Run("nothing attached is one line", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1")
		out := run(t, inst, []usbip.DeviceInfo{uart}, uartBusid)
		assert.Equal(t, out, `usb: device is not attached to instance "vm1"`)
		assert.Equal(t, run(t, inst, nil, "", "0403", "6015"), out)
	})

	t.Run("held runtime grant", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1")
		other := usbip.AllowEntry{VendorID: "1111", ProductID: "2222"}
		assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{other, grant}))
		endOnRevoke(holdDevice(t, uartBusid, "vm1"))

		out := run(t, inst, []usbip.DeviceInfo{uart}, "", "403", "6015")
		assert.Equal(t, out, "usb: detached "+uartBusid+` from instance "vm1"`)
		_, held := usbip.Holder(uartBusid)
		assert.Assert(t, !held)
		list, err := usbip.ReadAllowlist(inst.Dir)
		assert.NilError(t, err)
		assert.DeepEqual(t, list, []usbip.AllowEntry{other})
	})

	t.Run("unplugged device with stale grant", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1")
		assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{grant}))
		assert.Assert(t, strings.HasPrefix(run(t, inst, nil, "", "0403", "6015"), "usb: detached"))
		list, err := usbip.ReadAllowlist(inst.Dir)
		assert.NilError(t, err)
		assert.Equal(t, len(list), 0)
	})

	t.Run("stopped VM holds no lock", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1")
		assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{grant}))
		assert.Assert(t, strings.HasPrefix(run(t, inst, []usbip.DeviceInfo{uart}, uartBusid), "usb: detached"))
	})

	t.Run("yaml device held", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1", limatype.USBDevice{VendorID: "0403", ProductID: "6015"})
		seed := usbip.ConfigEntries(limayaml.USB(inst.Config).Devices)
		assert.NilError(t, usbip.WriteAllowlist(inst.Dir, seed))
		endOnRevoke(holdDevice(t, uartBusid, "vm1"))

		out := run(t, inst, []usbip.DeviceInfo{uart}, "", "0403", "6015")
		assert.Assert(t, strings.HasPrefix(out, "usb: detached "+uartBusid), out)
		assert.Assert(t, strings.Contains(out, "returns at next VM start unless removed"), out)
		assert.Equal(t, hook.LastEntry().Level, logrus.WarnLevel)
		list, err := usbip.ReadAllowlist(inst.Dir)
		assert.NilError(t, err)
		assert.Assert(t, !usbip.Allowed(list, usbip.EntryFor(uart)), "guest root can re-import")
		twin := uart
		twin.Busid = "1-0x01200000"
		assert.Assert(t, usbip.Allowed(list, usbip.EntryFor(twin)))

		assert.Assert(t, strings.Contains(run(t, inst, []usbip.DeviceInfo{uart}, "", "0403", "6015"), "not attached"))
	})

	t.Run("other instance keeps its device", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1")
		holdDevice(t, uartBusid, "vm2")
		assert.Assert(t, strings.Contains(run(t, inst, []usbip.DeviceInfo{uart}, uartBusid), "not attached"))
	})

	t.Run("identical devices need a busid", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1")
		twin := uart
		twin.Busid = "1-0x01200000"
		assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{grant, usbip.EntryFor(twin)}))

		assert.ErrorContains(t, detachDevice(inst, nil, "", "0403", "6015"), "multiple devices")

		run(t, inst, nil, twin.Busid)
		list, err := usbip.ReadAllowlist(inst.Dir)
		assert.NilError(t, err)
		assert.DeepEqual(t, list, []usbip.AllowEntry{grant})
	})

	t.Run("bad arguments", func(t *testing.T) {
		t.Setenv("LIMA_HOME", t.TempDir())
		inst := newUSBTestInstance(t, "vm1")
		assert.ErrorContains(t, detachDevice(inst, nil, "", "", ""), "specify a BUSID")
		assert.ErrorContains(t, detachDevice(inst, nil, "", "zz", "6015"), "invalid --vendor")
	})
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
	free := usbip.DeviceInfo{Vendor: 0x1111, Product: 0x2222, Busid: "2-0x02100000", VendorName: "ACME"}

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

func TestBusidRefusesVendorAndProduct(t *testing.T) {
	for _, cmd := range []*cobra.Command{newUsbAttachCommand(), newUsbDetachCommand()} {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetArgs([]string{"vm1", uartBusid, "--vendor", "0403", "--product", "6015"})
		assert.ErrorContains(t, cmd.Execute(), "not both")
	}
}

func TestMissingInstance(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	for _, cmd := range []*cobra.Command{newUsbListCommand(), newUsbAttachCommand(), newUsbDetachCommand()} {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetArgs([]string{"nope"})
		err := cmd.Execute()
		assert.ErrorContains(t, err, "does not exist")
		assert.Assert(t, !strings.Contains(err.Error(), "lima.yaml"), err.Error())
	}
}

func TestBusidCompletions(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	free := usbip.DeviceInfo{Vendor: 0x1111, Product: 0x2222, Busid: "2-0x02100000"}
	holdDevice(t, uartBusid, "vm1")
	hosts := []usbip.DeviceInfo{uart, free}

	assert.DeepEqual(t, busidCompletions(hosts, ""), []cobra.Completion{uartBusid + "\tUART", free.Busid + "\t1111:2222"})
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

func TestDetachCommandOnStoppedVM(t *testing.T) {
	t.Setenv("LIMA_HOME", t.TempDir())
	inst := newStoppedInstance(t, "vmType: vz\nvmOpts: {vz: {usb: {enabled: true}}}")
	assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{usbip.EntryFor(uart)}))

	hook := logrustest.NewGlobal()
	cmd := newUsbDetachCommand()
	cmd.SetArgs([]string{"vm1", uartBusid})
	assert.NilError(t, cmd.Execute())
	assert.Assert(t, strings.HasPrefix(hook.LastEntry().Message, "usb: detached "+uartBusid), hook.LastEntry().Message)
}

func TestRequireVZUSBInstanceGates(t *testing.T) {
	for config, want := range map[string]string{
		"vmType: vz\nvmOpts: {vz: {usb: {enabled: true}}}":                                                        "is not running",
		"vmType: vz\nvmOpts: {vz: {usb: {devices: [{vendorID: \"0403\", productID: \"6015\"}]}}}":                 "is not running",
		"vmType: vz\nvmOpts: {vz: {usb: {enabled: false}}}":                                                       "does not have USB enabled",
		"vmType: vz\nvmOpts: {vz: {usb: {enabled: false, devices: [{vendorID: \"0403\", productID: \"6015\"}]}}}": "does not have USB enabled",
		"vmType: qemu\nvmOpts: {vz: {usb: {enabled: true}}}":                                                      "is not a vz instance",
		"vmType: vz\nvmOpts: {vz: {usb: {enabled: true}}}\nplain: true\nmountType: reverse-sshfs":                 "plain mode",
		"vmType: vz\nos: Darwin\nvmOpts: {vz: {usb: {enabled: true}}}":                                            "not a Linux guest",
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
	inst := newStoppedInstance(t, "vmType: vz\nvmOpts: {vz: {usb: {enabled: true}}}")
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
