// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package usbip

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// One lost control read costs 5 s before the hub core retries.
const enumerateTimeout = 15 * time.Second

// A raw blocking socket, because vhci takes the fd number.
func dialHost() (*os.File, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return nil, fmt.Errorf("creating vsock socket: %w", err)
	}
	if err := unix.Connect(fd, &unix.SockaddrVM{CID: unix.VMADDR_CID_HOST, Port: VsockPort}); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("host USB server unreachable, set `vmOpts.vz.usb.enabled: true` and restart the instance: %w", err)
	}
	return os.NewFile(uintptr(fd), "vsock"), nil
}

// Probe reports whether the host serves USB/IP, which it does only with `vmOpts.vz.usb.enabled`.
// Closing before any request ends the host session at once, so no connection slot stays held.
func Probe() error {
	conn, err := dialHost()
	if err != nil {
		return err
	}
	return conn.Close()
}

func AttachBusid(busid string) error {
	conn, err := dialHost()
	if err != nil {
		return err
	}
	// vhci keeps its own reference to the socket.
	defer conn.Close()

	if err := writeOpHeader(conn, opReqImport, 0); err != nil {
		return fmt.Errorf("sending import request: %w", err)
	}
	var b [32]byte
	copy(b[:], busid)
	if _, err := conn.Write(b[:]); err != nil {
		return fmt.Errorf("sending import busid: %w", err)
	}
	h, err := readOpHeader(conn)
	if err != nil {
		return fmt.Errorf("reading import reply: %w", err)
	}
	if h.Status != 0 {
		return fmt.Errorf("host refused import of %s; see ha.stderr.log on the host", busid)
	}
	var desc usbDeviceDesc
	if err := binary.Read(conn, binary.BigEndian, &desc); err != nil {
		return fmt.Errorf("reading device descriptor: %w", err)
	}

	port, err := attachPort(int(conn.Fd()), desc.Busnum<<16|desc.Devnum, desc.Speed)
	if err != nil {
		return fmt.Errorf("attaching %s to vhci: %w", busid, err)
	}
	return waitEnumerated(port, busid, enumerateTimeout)
}

func AttachAll() error {
	busids, err := hostDevlist()
	if err != nil {
		return err
	}
	for _, b := range busids {
		if err := AttachBusid(b); err != nil {
			logrus.WithError(err).Warnf("usbip: attaching %s failed", b)
		}
	}
	return nil
}

func hostDevlist() ([]string, error) {
	conn, err := dialHost()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := writeOpHeader(conn, opReqDevlist, 0); err != nil {
		return nil, fmt.Errorf("sending devlist request: %w", err)
	}
	h, err := readOpHeader(conn)
	if err != nil {
		return nil, fmt.Errorf("reading devlist reply: %w", err)
	}
	if h.Status != 0 {
		return nil, fmt.Errorf("host devlist returned status %d", h.Status)
	}
	var count uint32
	if err := binary.Read(conn, binary.BigEndian, &count); err != nil {
		return nil, fmt.Errorf("reading devlist count: %w", err)
	}
	var busids []string
	for i := range count {
		var desc usbDeviceDesc
		if err := binary.Read(conn, binary.BigEndian, &desc); err != nil {
			return nil, fmt.Errorf("reading devlist entry %d: %w", i, err)
		}
		busids = append(busids, cString(desc.Busid[:]))
	}
	return busids, nil
}
