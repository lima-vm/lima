// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usbip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// VDEV_ST_NULL of enum usbip_device_status.
const vdevStNull = 4

var (
	vhciDir        = "/sys/devices/platform/vhci_hcd.0"
	usbDevicesPath = "/sys/bus/usb/devices"
	writeSysfs     = func(path, data string) error { return os.WriteFile(path, []byte(data), 0) }
)

func statusLines() ([][]string, error) {
	data, err := os.ReadFile(filepath.Join(vhciDir, "status"))
	if err != nil {
		return nil, fmt.Errorf("reading vhci status (vhci-hcd not loaded; the guest kernel may lack it, see the USB passthrough docs): %w", err)
	}
	var out [][]string
	for line := range strings.SplitSeq(string(data), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			out = append(out, f)
		}
	}
	return out, nil
}

func freePort(speed uint32) (int, error) {
	wantHub := "hs"
	if speed == usbipSpeedSuper {
		wantHub = "ss"
	}
	lines, err := statusLines()
	if err != nil {
		return 0, err
	}
	for _, f := range lines {
		if len(f) < 3 || f[0] != wantHub {
			continue
		}
		if sta, err := strconv.Atoi(f[2]); err != nil || sta != vdevStNull {
			continue
		}
		if port, err := strconv.Atoi(f[1]); err == nil {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free vhci %s port available", wantHub)
}

// EBUSY means another attach won the port: try the next, as usbip_attach.c does.
func attachPort(fd int, devid, speed uint32) (int, error) {
	for {
		port, err := freePort(speed)
		if err != nil {
			return 0, err
		}
		line := fmt.Sprintf("%d %d %d %d", port, fd, devid, speed)
		if err := writeSysfs(filepath.Join(vhciDir, "attach"), line); !errors.Is(err, syscall.EBUSY) {
			return port, err
		}
	}
}

func portEnumerated(port int) bool {
	lines, err := statusLines()
	if err != nil {
		return false
	}
	for _, f := range lines {
		if len(f) < 4 {
			continue
		}
		if num, err := strconv.Atoi(f[1]); err != nil || num != port {
			continue
		}
		// The kernel prints 0-0 here until the device has an address.
		_, err = os.Stat(filepath.Join(usbDevicesPath, f[len(f)-1]))
		return err == nil
	}
	return false
}

// On timeout it releases the port, so the device never sits half attached.
func waitEnumerated(port int, busid string, timeout time.Duration) error {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if portEnumerated(port) {
			return nil
		}
	}
	_ = writeSysfs(filepath.Join(vhciDir, "detach"), strconv.Itoa(port))
	return fmt.Errorf("%s did not enumerate on vhci port %d within %s; see dmesg", busid, port, timeout)
}
