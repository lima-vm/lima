// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package osutil

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/plist"
)

var MachineID = sync.OnceValue(func() string {
	x, err := machineID(context.Background())
	if err == nil && x != "" {
		return x
	}
	logrus.WithError(err).Debug("failed to get machine ID, falling back to use hostname instead")
	hostname, err := os.Hostname()
	if err != nil {
		panic(fmt.Errorf("failed to get hostname: %w", err))
	}
	return hostname
})

func machineID(ctx context.Context) (string, error) {
	if runtime.GOOS == "darwin" {
		ioPlatformExpertDeviceCmd := exec.CommandContext(ctx, "/usr/sbin/ioreg", "-a", "-d2", "-c", "IOPlatformExpertDevice")
		ioPlatformExpertDevice, err := ioPlatformExpertDeviceCmd.CombinedOutput()
		if err != nil {
			return "", err
		}
		return parseIOPlatformUUIDFromIOPlatformExpertDevice(bytes.NewReader(ioPlatformExpertDevice))
	}

	candidates := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
		// We don't use "/sys/class/dmi/id/product_uuid"
	}
	for _, f := range candidates {
		b, err := os.ReadFile(f)
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
	}
	return "", fmt.Errorf("no machine-id found, tried %v", candidates)
}

func parseIOPlatformUUIDFromIOPlatformExpertDevice(r io.Reader) (string, error) {
	var p plist.Plist
	dec := xml.NewDecoder(r)
	if err := dec.Decode(&p); err != nil {
		return "", err
	}
	if p.Value.Dict == nil {
		return "", errors.New("invalid plist: top-level value is not a dict")
	}
	ioRegistryEntryChildren, ok := p.Value.Dict["IORegistryEntryChildren"]
	if !ok || ioRegistryEntryChildren.Array == nil || len(ioRegistryEntryChildren.Array) == 0 {
		return "", errors.New("invalid plist: IORegistryEntryChildren not found or empty")
	}
	for _, child := range ioRegistryEntryChildren.Array {
		if child.Dict == nil {
			continue
		}
		ioPlatformUUID, ok := child.Dict["IOPlatformUUID"]
		if !ok || ioPlatformUUID.String == nil {
			continue
		}
		return *ioPlatformUUID.String, nil
	}

	return "", errors.New("invalid plist: IOPlatformUUID not found in any child of IORegistryEntryChildren")
}
