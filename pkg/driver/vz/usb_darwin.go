//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/lockutil"
	"github.com/lima-vm/lima/v2/pkg/usbip"
)

// maxUSBIPConns is twice vhci's default 16 ports; raise it if a guest needs more.
const maxUSBIPConns = 32

// usbDriver runs the USB/IP server for the VM's lifetime, so vm_darwin.go needs no USB/IP code.
type usbDriver struct {
	*LimaVzDriver
	usbip func()
}

func (d *usbDriver) Configure(ctx context.Context, inst *limatype.Instance) (*driver.ConfiguredDriver, error) {
	cd, err := d.LimaVzDriver.Configure(ctx, inst)
	if err != nil {
		return nil, err
	}
	cd.Driver = d
	return cd, nil
}

func (d *usbDriver) Start(ctx context.Context) (chan error, error) {
	errCh, err := d.LimaVzDriver.Start(ctx)
	if err != nil {
		return nil, err
	}
	if d.usbip, err = d.machine.startUSBIPServer(ctx, d.Instance); err != nil {
		logrus.WithError(err).Warn("Failed to start USB/IP passthrough server")
	}
	return errCh, nil
}

// Stop returns nil only once the VM stopped, and Listener.Close panics if a guest connects during it.
func (d *usbDriver) Stop(ctx context.Context) error {
	err := d.LimaVzDriver.Stop(ctx)
	if err == nil {
		d.stopUSBIP()
	}
	return err
}

func (d *usbDriver) stopUSBIP() {
	if d.usbip == nil {
		return
	}
	d.usbip()
	if err := seedAllowlist(d.Instance); err != nil {
		logrus.WithError(err).Warn("Failed to reset USB/IP allowlist")
	}
}

func (m *virtualMachineWrapper) startUSBIPServer(ctx context.Context, inst *limatype.Instance) (func(), error) {
	usb := inst.Config.USB
	if usb.Enabled == nil || !*usb.Enabled || *inst.Config.Plain || *inst.Config.OS != limatype.LINUX {
		return nil, nil
	}

	if err := seedAllowlist(inst); err != nil {
		return nil, fmt.Errorf("seeding usbip allowlist: %w", err)
	}
	if hosts, err := usbip.List(); err == nil {
		warnUnmatchedDevices(hosts, usb.Devices)
	}
	server := usbip.NewServer(usbip.NewProvider(inst.Dir))

	socketDevices := m.SocketDevices()
	if len(socketDevices) == 0 {
		return nil, errors.New("no vsock device available for USB/IP passthrough")
	}
	listener, err := socketDevices[0].Listen(usbip.VsockPort)
	if err != nil {
		return nil, fmt.Errorf("listening on vsock port %d: %w", usbip.VsockPort, err)
	}

	// The server outlives ctx: only usbDriver.Stop may end it.
	ctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	context.AfterFunc(ctx, func() { _ = listener.Close() })

	go acceptUSBIP(ctx, listener, server)

	logrus.Infof("usbip: serving allowlisted USB devices on vsock port %d", usbip.VsockPort)
	return cancel, nil
}

func acceptUSBIP(ctx context.Context, l net.Listener, s *usbip.Server) {
	slots := make(chan struct{}, maxUSBIPConns)
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logrus.WithError(err).Warn("usbip: vsock accept error")
			continue
		}
		select {
		case slots <- struct{}{}:
		default:
			logrus.Debug("usbip: too many connections")
			_ = conn.Close()
			continue
		}
		go func() {
			defer func() { <-slots }()
			defer conn.Close()
			stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
			defer stop()
			err := s.Serve(ctx, conn)
			if err != nil && ctx.Err() == nil && !errors.Is(err, usbip.ErrDeviceGone) {
				logrus.WithError(err).Warn("usbip: session ended")
			}
		}()
	}
}

// YAML alone, so runtime grants end with the VM.
func seedAllowlist(inst *limatype.Instance) error {
	return lockutil.WithDirLock(inst.Dir, func() error {
		return usbip.WriteAllowlist(inst.Dir, usbip.ConfigEntries(inst.Config.USB.Devices))
	})
}

func warnUnmatchedDevices(hosts []usbip.DeviceInfo, devs []limatype.USBDevice) {
	for _, want := range usbip.ConfigEntries(devs) {
		if !slices.ContainsFunc(hosts, func(h usbip.DeviceInfo) bool {
			return usbip.Allowed([]usbip.AllowEntry{want}, usbip.EntryFor(h))
		}) {
			logrus.Warnf("usbip: no host USB device matches usb.devices entry %s:%s", want.VendorID, want.ProductID)
		}
	}
}
