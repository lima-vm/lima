//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/registry"
	"github.com/lima-vm/lima/v2/pkg/usbip"
)

// fakeListener returns an Accept error for a nil conn, as vz does.
type fakeListener struct {
	net.Listener // only Accept is used
	conns        chan net.Conn
}

func (l *fakeListener) Accept() (net.Conn, error) {
	c := <-l.conns
	if c == nil {
		return nil, errors.New("accept failed")
	}
	return c, nil
}

func TestAcceptUSBIP(t *testing.T) {
	hook := logrustest.NewGlobal()
	warns := func() int {
		n := 0
		for _, e := range hook.AllEntries() {
			if e.Level == logrus.WarnLevel {
				n++
			}
		}
		return n
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	l := &fakeListener{conns: make(chan net.Conn)}
	done := make(chan struct{})
	go func() {
		acceptUSBIP(ctx, l, usbip.NewServer(nil))
		close(done)
	}()

	push := func(c net.Conn) {
		select {
		case l.conns <- c:
		case <-time.After(5 * time.Second):
			assert.Assert(t, false, "accept loop stopped")
		}
	}

	baseline := runtime.NumGoroutine()
	push(nil)       // vz accept error: warn, keep accepting
	for range 100 { // guests hanging up is no failure, and their sessions end
		c, s := net.Pipe()
		push(s)
		assert.NilError(t, c.Close())
	}
	deadline := time.Now().Add(5 * time.Second) // not poll: it checks in a goroutine of its own
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Assert(t, runtime.NumGoroutine() <= baseline, "goroutines %d, baseline %d", runtime.NumGoroutine(), baseline)

	bad, srv := net.Pipe()
	push(srv)
	_, err := bad.Write([]byte{0x01, 0x11, 0xff, 0xff, 0, 0, 0, 0})
	assert.NilError(t, err)
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if warns() >= 2 {
			return poll.Success()
		}
		return poll.Continue("%d warnings", warns())
	}, poll.WithDelay(5*time.Millisecond))

	idle, srv := net.Pipe()
	assert.NilError(t, idle.SetReadDeadline(time.Now().Add(5*time.Second)))
	push(srv)
	cancel()
	_, err = idle.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
	close(l.conns)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "accept loop did not stop")
	}
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, warns(), 2, "VM stop is no failure")
}

func TestAcceptUSBIPCapsConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	l := &fakeListener{conns: make(chan net.Conn)}
	go acceptUSBIP(ctx, l, usbip.NewServer(nil))
	t.Cleanup(func() { close(l.conns) })

	idle := make([]net.Conn, maxUSBIPConns)
	for i := range idle {
		var srv net.Conn
		idle[i], srv = net.Pipe()
		l.conns <- srv
	}
	over, srv := net.Pipe()
	assert.NilError(t, over.SetReadDeadline(time.Now().Add(5*time.Second)))
	l.conns <- srv
	_, err := over.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)

	// A free slot takes a new conn: EOF would come at once if it were dropped.
	assert.NilError(t, idle[0].Close())
	deadline := time.Now().Add(5 * time.Second)
	for {
		next, srv := net.Pipe()
		assert.NilError(t, next.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
		l.conns <- srv
		_, err := next.Read(make([]byte, 1))
		if errors.Is(err, os.ErrDeadlineExceeded) {
			break
		}
		assert.Assert(t, time.Now().Before(deadline), "slot never freed: %v", err)
	}
}

func usbConfig(usb limatype.USB) *limatype.LimaYAML {
	return &limatype.LimaYAML{VMOpts: limatype.VMOpts{limatype.VZ: limatype.VZOpts{USB: usb}}}
}

func TestStartUSBIPServerSkips(t *testing.T) {
	for name, set := range map[string]func(*limatype.LimaYAML){
		"plain":     func(y *limatype.LimaYAML) { y.Plain = new(true) },
		"not Linux": func(y *limatype.LimaYAML) { y.Plain, y.OS = new(false), new(limatype.DARWIN) },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := usbConfig(limatype.USB{Enabled: new(true)})
			set(cfg)
			var m *virtualMachineWrapper // Serving would dereference it.
			stop, err := m.startUSBIPServer(t.Context(), &limatype.Instance{Dir: t.TempDir(), Config: cfg})
			assert.NilError(t, err)
			assert.Assert(t, stop == nil)
		})
	}
}

// Stop resets the allowlist to the YAML entries, so runtime grants end with the VM.
func TestStopUSBIP(t *testing.T) {
	grant := usbip.EntryFor(usbip.DeviceInfo{Vendor: 0x403, Product: 0x6015, Busid: "1-0x01100000"})
	for name, tc := range map[string]struct {
		devices []limatype.USBDevice
		want    []usbip.AllowEntry
	}{
		"no yaml": {nil, []usbip.AllowEntry{}},
		"yaml": {
			[]limatype.USBDevice{{Name: "uart", VendorID: "0403", ProductID: "6015"}},
			[]usbip.AllowEntry{{Name: "uart", VendorID: "0403", ProductID: "6015"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			inst := &limatype.Instance{Dir: t.TempDir(), Config: usbConfig(limatype.USB{Devices: tc.devices})}
			assert.NilError(t, usbip.WriteAllowlist(inst.Dir, []usbip.AllowEntry{grant}))
			d := &usbDriver{LimaVzDriver: &LimaVzDriver{Instance: inst}}
			d.stopUSBIP() // USB never started
			stopped := false
			d.usbip = func() { stopped = true }
			d.stopUSBIP()
			assert.Assert(t, stopped)
			got, err := usbip.ReadAllowlist(inst.Dir)
			assert.NilError(t, err)
			assert.DeepEqual(t, got, tc.want)
		})
	}
}

func TestSeedAllowlistIgnoresUnparsableFile(t *testing.T) {
	inst := &limatype.Instance{Dir: t.TempDir(), Config: &limatype.LimaYAML{}}
	assert.NilError(t, os.WriteFile(filepath.Join(inst.Dir, "usbip-allow.json"), []byte("{"), 0o600))
	assert.NilError(t, seedAllowlist(inst))
	got, err := usbip.ReadAllowlist(inst.Dir)
	assert.NilError(t, err)
	assert.Equal(t, len(got), 0)
}

func TestWarnUnmatchedDevices(t *testing.T) {
	hook := logrustest.NewGlobal()
	uart := usbip.DeviceInfo{Vendor: 0x403, Product: 0x6015, Busid: "1-0x01100000"}
	warnUnmatchedDevices([]usbip.DeviceInfo{uart}, []limatype.USBDevice{
		{VendorID: "0403", ProductID: "6015"},
		{VendorID: "0403", ProductID: "6015", BusAddr: uart.Busid},
		{VendorID: "0403", ProductID: "6015", BusAddr: "2-0x02100000"},
		{VendorID: "0bda", ProductID: "8812"},
	})
	var got []string
	for _, e := range hook.AllEntries() {
		got = append(got, e.Message)
	}
	assert.DeepEqual(t, got, []string{
		"usbip: no host USB device matches vmOpts.vz.usb.devices entry 0403:6015",
		"usbip: no host USB device matches vmOpts.vz.usb.devices entry 0bda:8812",
	})
}

// The vz registration must be the USB wrapper, and Configure's vmOpts rewrite must keep usb as is.
func TestUSBDriverValidate(t *testing.T) {
	for usb, wantErr := range map[string]string{
		`{devices: [{vendorID: "0403", productID: "6015"}]}`: "",
		`{devices: [{vendorID: 0x0403, productID: "6015"}]}`: "vendorID` must be 4 hex digits in quotes, got 1027",
	} {
		t.Run(usb, func(t *testing.T) {
			_, d, _ := registry.Get("vz")
			dir := t.TempDir()
			y := "{images: [{location: /}], vmType: vz, mountType: virtiofs, vmOpts: {vz: {usb: " + usb + "}}}"
			cfg, err := limayaml.Load(t.Context(), []byte(y), filepath.Join(dir, "lima.yaml"))
			assert.NilError(t, err)
			cd, err := d.Configure(t.Context(), &limatype.Instance{Dir: dir, Config: cfg})
			assert.NilError(t, err)
			err = cd.Validate(t.Context())
			if wantErr == "" {
				assert.NilError(t, err)
			} else {
				assert.ErrorContains(t, err, wantErr)
			}
		})
	}
}

func TestUSBIPBootScript(t *testing.T) {
	scripts, err := New().BootScripts(t.Context())
	assert.NilError(t, err)
	script, ok := scripts["boot.Linux/34-usbip-attach.sh"]
	assert.Assert(t, ok)

	stubs := map[string]string{
		"lima-guestagent": `echo "ga $*" >>"$LOG"`,
		"modprobe":        `echo modprobe >>"$LOG"; [ -e "$DIR/module" ]`,
		"timeout":         `echo "timeout $1" >>"$LOG"; shift; exec "$@"`,
		"apt-get":         `echo "apt-get $1" >>"$LOG"; [ "$1" = update ] || touch "$DIR/module"`,
		"uname":           `echo 6.8.0`,
	}
	const ubuntu, debian = "ID=ubuntu", "ID=debian"
	apt := "modprobe,timeout 120,apt-get update,apt-get install,modprobe"
	for name, tc := range map[string]struct {
		osRelease, skip string
		module, warn    bool
		want            string
	}{
		"module present": {osRelease: ubuntu, module: true, want: "modprobe"},
		"ubuntu":         {osRelease: ubuntu, want: apt},
		"debian":         {osRelease: debian, warn: true, want: "modprobe"},
		"ubuntu, skip":   {osRelease: ubuntu, skip: "1", warn: true, want: "modprobe"},
		"no os-release":  {warn: true, want: "modprobe"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			assert.NilError(t, os.Mkdir(filepath.Join(dir, "bin"), 0o755))
			for stub, body := range stubs {
				assert.NilError(t, os.WriteFile(filepath.Join(dir, "bin", stub), []byte("#!/bin/sh\n"+body+"\n"), 0o755))
			}
			if tc.module {
				assert.NilError(t, os.WriteFile(filepath.Join(dir, "module"), nil, 0o644))
			}
			osRelease := filepath.Join(dir, "os-release")
			if tc.osRelease != "" {
				assert.NilError(t, os.WriteFile(osRelease, []byte(tc.osRelease), 0o644))
			}
			log := filepath.Join(dir, "log")
			cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", strings.ReplaceAll(string(script), "/etc/os-release", osRelease))
			cmd.Env = []string{
				"PATH=" + filepath.Join(dir, "bin") + ":/usr/bin:/bin", "DIR=" + dir, "LOG=" + log,
				"LIMA_CIDATA_GUEST_INSTALL_PREFIX=" + dir, "LIMA_CIDATA_SKIP_DEFAULT_DEPENDENCY_RESOLUTION=" + tc.skip,
			}
			out, err := cmd.CombinedOutput()
			assert.NilError(t, err, string(out))
			assert.Equal(t, strings.Contains(string(out), "WARNING: no vhci-hcd"), tc.warn, string(out))
			calls, err := os.ReadFile(log)
			assert.NilError(t, err)
			assert.Equal(t, strings.ReplaceAll(strings.TrimSpace(string(calls)), "\n", ","), "ga usbip probe,"+tc.want+",ga usbip attach --all")
		})
	}
}
