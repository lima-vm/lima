// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hcs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/lima-vm/go-qcow2reader/image/qcow2"
	"github.com/lima-vm/go-qcow2reader/image/vhdx"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/imgutil/nativeimgutil"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
)

const (
	// hcsTimeoutInfinite corresponds to Win32 INFINITE.
	hcsTimeoutInfinite uint32 = 0xFFFFFFFF

	// These flags are not exported by the hcn package.
	netFlagEnableDNS     hcn.NetworkFlags = 1
	netFlagEnableDHCP    hcn.NetworkFlags = 2
	netFlagIsolateSwitch hcn.NetworkFlags = 32

	networkAddressPrefix = "192.168.10.0/24"
	networkGateway       = "192.168.10.1"
)

type computeSystemProperties struct {
	State string `json:"State"`
}

var errNotFound = errors.New("not found")

// getComputeSystemState obtains compute system properties.
func getComputeSystemState(ctx context.Context, id string) (string, error) {
	system, err := hcsOpenComputeSystem(ctx, id, syscall.GENERIC_ALL)
	if err != nil {
		return "", errNotFound
	}
	defer hcsCloseComputeSystem(system)

	op, err := hcsCreateOperation()
	if err != nil {
		return "", fmt.Errorf("failed to create an HCS operation: %w", err)
	}
	defer hcsCloseOperation(op)

	if err := hcsGetComputeSystemProperties(ctx, system, op, "{}"); err != nil {
		return "", fmt.Errorf("failed to get compute system properties: %w", err)
	}

	result, err := hcsWait(op, "hcsGetComputeSystemProperties", nil)
	if err != nil {
		return "", fmt.Errorf("failed to get compute system properties: %w (result=%s)", err, result)
	}

	var properties computeSystemProperties
	if err := json.Unmarshal([]byte(result), &properties); err != nil {
		return "", fmt.Errorf("failed to decode compute system properties: %w", err)
	}

	return properties.State, nil
}

// buildVMConfig returns the HCS compute system configuration document.
func buildVMConfig(
	diskPath, isoPath, endpointID, macAddress, serialPipe string,
	cpus int, memoryMiB int64,
) (string, error) {
	config := map[string]any{
		"SchemaVersion": map[string]any{
			"Major": 2,
			"Minor": 1,
		},
		"Owner":                             "Lima",
		"ShouldTerminateOnLastHandleClosed": true,
		"VirtualMachine": map[string]any{
			"Chipset": map[string]any{
				"Uefi": map[string]any{
					"Console": "ComPort1",
					"BootThis": map[string]any{
						"DevicePath": "Primary disk",
						"DiskNumber": 0,
						"DeviceType": "ScsiDrive",
					},
				},
			},
			"ComputeTopology": map[string]any{
				"Memory": map[string]any{
					"Backing":  "Virtual",
					"SizeInMB": memoryMiB,
				},
				"Processor": map[string]any{
					"Count": cpus,
				},
			},
			"Devices": map[string]any{
				"Scsi": map[string]any{
					"Primary disk": map[string]any{
						"Attachments": map[string]any{
							"0": map[string]any{
								"Type": "VirtualDisk",
								"Path": diskPath,
							},
						},
					},
					"cidata": map[string]any{
						"Attachments": map[string]any{
							"0": map[string]any{
								"Type": "Iso",
								"Path": isoPath,
							},
						},
					},
				},
				"ComPorts": map[string]any{
					"0": map[string]any{
						"NamedPipe": serialPipe,
					},
				},
				"NetworkAdapters": map[string]any{
					endpointID: map[string]any{
						"EndpointId": endpointID,
						"MacAddress": macAddress,
					},
				},
			},
		},
	}

	configBytes, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	return string(configBytes), nil
}

func ensureNetwork(name string) (*hcn.HostComputeNetwork, error) {
	network, err := hcn.GetNetworkByName(name)
	if err == nil {
		return network, nil
	}

	if !hcn.IsNotFoundError(err) {
		return nil, fmt.Errorf("failed to look up the HCN network %#q: %w", name, err)
	}

	network = &hcn.HostComputeNetwork{
		Name:          name,
		SchemaVersion: hcn.V2SchemaVersion(),
		Type:          hcn.ICS,
		Flags:         netFlagEnableDNS | netFlagEnableDHCP | hcn.EnableNonPersistent | netFlagIsolateSwitch,
		Ipams: []hcn.Ipam{{
			Type: "Static",
			Subnets: []hcn.Subnet{{
				IpAddressPrefix: networkAddressPrefix,
				Routes: []hcn.Route{{
					NextHop:           networkGateway,
					DestinationPrefix: "0.0.0.0/0",
				}},
			}},
		}},
	}

	return network.Create()
}

func ensureEndpoint(network *hcn.HostComputeNetwork, name, instDir string) (*hcn.HostComputeEndpoint, error) {
	endpoint, err := hcn.GetEndpointByName(name)
	if err == nil {
		if endpoint.HostComputeNetwork != network.Id {
			return nil, fmt.Errorf("endpoint %#q already exists on a different network", name)
		}
		return endpoint, nil
	}
	if !hcn.IsNotFoundError(err) {
		return nil, err
	}

	endpoint = &hcn.HostComputeEndpoint{
		Name:               name,
		SchemaVersion:      hcn.V2SchemaVersion(),
		HostComputeNetwork: network.Id,
		MacAddress:         hnsMACAddress(instDir),
	}

	return endpoint.Create()
}

func networkName(instName string) string {
	return "lima-" + instName + "-net"
}

func endpointName(instName string) string {
	return "lima-" + instName + "-ep"
}

// hnsMACAddress converts MAC address notation saved in limayaml struct to the one HNS expects.
func hnsMACAddress(instDir string) string {
	return strings.ToUpper(strings.ReplaceAll(limayaml.MACAddress(instDir), ":", "-"))
}

// createComputeSystem creates (but does not start) an HCS compute system.
func createComputeSystem(_ context.Context, id, config string) (hcsSystem, error) {
	system, err := hcsCreateComputeSystem(id, config)
	if err != nil {
		return 0, fmt.Errorf("create compute system error: %w", err)
	}

	return system, nil
}

func startComputeSystem(_ context.Context, system hcsSystem) error {
	return hcsStartComputeSystem(system)
}

func terminateComputeSystem(_ context.Context, system hcsSystem) error {
	return hcsTerminateComputeSystem(system)
}

// serveSerial accepts connections from the guest port.
// It appends everything it reads to logPath and return when ln is closed.
func serveSerial(ln net.Listener, logPath string) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		logrus.WithError(err).Errorf("failed to open the serial log %#q", logPath)
		return
	}
	defer f.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			logrus.WithError(err).Warn("failed to accept a serial connection")
			return
		}
		logrus.Debugf("serial: the guest connected to %s", ln.Addr())
		if _, err := io.Copy(f, conn); err != nil {
			logrus.WithError(err).Debug("serial: copy finished with an error")
		}
		conn.Close()
	}
}

// mayConvertQcow2ToVHDX checks `disk` and converts it to .vhdx if it is qcow2.
func mayConvertQcow2ToVHDX(ctx context.Context, instDir string) error {
	disk := filepath.Join(instDir, filenames.Disk)
	_, err := os.Stat(disk)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}

	imagePath := filepath.Join(instDir, filenames.Image)
	format, err := nativeimgutil.DetectFormat(imagePath)
	if err != nil {
		return err
	}

	switch format {
	case string(vhdx.Type):
		logrus.Debug("disk is already .vhdx")
		if err = os.Rename(imagePath, disk); err != nil {
			return fmt.Errorf("failed to rename %#q to %#q: %w", imagePath, disk, err)
		}
		if err := os.Chmod(disk, 0o644); err != nil {
			return fmt.Errorf("failed to chmod %#q: %w", disk, err)
		}
		return nil
	case qcow2.Type, "raw":
		if err := execQemuImgConvert(ctx, imagePath, disk); err != nil {
			return fmt.Errorf("failed to convert to vhdx: %w", err)
		}
		if err := execFsutil(ctx, disk); err != nil {
			return fmt.Errorf("failed to set disk flag: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("disk type %s is not supported", format)
	}
}

func execQemuImgConvert(ctx context.Context, source, dist string) error {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-O", "vhdx", source, dist)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: stdout=%#q, stderr=%#q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	return nil
}

func execFsutil(ctx context.Context, disk string) error {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "fsutil", "sparse", "setflag", disk, "0")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: stdout=%#q, stderr=%#q: %w", cmd.Args, stdout.String(), stderr.String(), err)
	}
	return nil
}
