// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krunkit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/go-units"
	"github.com/lima-vm/go-qcow2reader/image/raw"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver/vz"
	"github.com/lima-vm/lima/v2/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/networks/usernet"
	"github.com/lima-vm/lima/v2/pkg/store"
)

const logLevelInfo = "3"

// Cmdline constructs the command line arguments for krunkit based on the instance configuration.
func Cmdline(inst *limatype.Instance) (*exec.Cmd, error) {
	memBytes, err := units.RAMInBytes(*inst.Config.Memory)
	if err != nil {
		return nil, err
	}

	args := []string{
		// Memory in MiB
		"--memory", strconv.FormatInt(memBytes/units.MiB, 10),
		"--cpus", fmt.Sprintf("%d", *inst.Config.CPUs),
		"--device", fmt.Sprintf("virtio-serial,logFilePath=%s", filepath.Join(inst.Dir, filenames.SerialLog)),
		"--krun-log-level", logLevelInfo,
		"--restful-uri", "none://",

		// First virtio-blk device is the boot disk
		"--device", fmt.Sprintf("virtio-blk,path=%s,format=raw", filepath.Join(inst.Dir, filenames.DiffDisk)),
		"--device", fmt.Sprintf("virtio-blk,path=%s", filepath.Join(inst.Dir, filenames.CIDataISO)),
	}

	// Add additional disks
	if len(inst.Config.AdditionalDisks) > 0 {
		ctx := context.Background()
		diskUtil := proxyimgutil.NewDiskUtil(ctx)
		for _, d := range inst.Config.AdditionalDisks {
			disk, derr := store.InspectDisk(d.Name, d.FSType)
			if derr != nil {
				return nil, fmt.Errorf("failed to load disk %q: %w", d.Name, derr)
			}
			if disk.Instance != "" {
				return nil, fmt.Errorf("failed to run attach disk %q, in use by instance %q", disk.Name, disk.Instance)
			}
			if lerr := disk.Lock(inst.Dir); lerr != nil {
				return nil, fmt.Errorf("failed to lock disk %q: %w", d.Name, lerr)
			}
			extraDiskPath := filepath.Join(disk.Dir, filenames.DataDisk)
			logrus.Infof("Mounting disk %q on %q", disk.Name, disk.MountPoint)
			if cerr := diskUtil.Convert(ctx, raw.Type, extraDiskPath, extraDiskPath, nil, true); cerr != nil {
				return nil, fmt.Errorf("failed to convert extra disk %q to raw: %w", extraDiskPath, cerr)
			}
			args = append(args, "--device", fmt.Sprintf("virtio-blk,path=%s,format=raw", extraDiskPath))
		}
	}

	// Network commands
	networkArgs, err := buildNetworkArgs(inst)
	if err != nil {
		return nil, fmt.Errorf("failed to build network arguments: %w", err)
	}

	// File sharing commands
	if *inst.Config.MountType == limatype.VIRTIOFS {
		for _, mount := range inst.Config.Mounts {
			if _, err := os.Stat(mount.Location); errors.Is(err, os.ErrNotExist) {
				if err := os.MkdirAll(mount.Location, 0o750); err != nil {
					return nil, err
				}
			}
			tag := limayaml.MountTag(mount.Location, *mount.MountPoint)
			mountArg := fmt.Sprintf("virtio-fs,sharedDir=%s,mountTag=%s", mount.Location, tag)
			args = append(args, "--device", mountArg)
		}
	}

	args = append(args, networkArgs...)
	cmd := exec.CommandContext(context.Background(), vmType, args...)

	return cmd, nil
}

func buildNetworkArgs(inst *limatype.Instance) ([]string, error) {
	var args []string

	// Configure default usernetwork with limayaml.MACAddress(inst.Dir) for eth0 interface
	firstUsernetIndex := limayaml.FirstUsernetIndex(inst.Config)
	if firstUsernetIndex == -1 {
		// slirp network using gvisor netstack
		krunkitSock, err := usernet.SockWithDirectory(inst.Dir, "", usernet.FDSock)
		if err != nil {
			return nil, err
		}
		client, err := vz.PassFDToUnix(krunkitSock)
		if err != nil {
			return nil, err
		}

		args = append(args, "--device", fmt.Sprintf("virtio-net,type=unixgram,fd=%d,mac=%s", client.Fd(), limayaml.MACAddress(inst.Dir)))
	}

	for _, nw := range inst.Networks {
		var sock string
		var mac string
		if nw.Lima != "" {
			nwCfg, err := networks.LoadConfig()
			if err != nil {
				return nil, err
			}
			switch nw.Lima {
			case networks.ModeUserV2:
				sock, err = usernet.Sock(nw.Lima, usernet.QEMUSock)
				if err != nil {
					return nil, err
				}
				mac = limayaml.MACAddress(inst.Dir)
			case networks.ModeShared, networks.ModeBridged:
				socketVMNetInstalled, err := nwCfg.IsDaemonInstalled(networks.SocketVMNet)
				if err != nil {
					return nil, err
				}
				if !socketVMNetInstalled {
					return nil, errors.New("socket_vmnet is not installed")
				}
				sock, err = networks.Sock(nw.Lima)
				if err != nil {
					return nil, err
				}
				mac = nw.MACAddress
			default:
				return nil, fmt.Errorf("invalid network spec %+v", nw)
			}
		} else if nw.Socket != "" {
			sock = nw.Socket
			mac = nw.MACAddress
		} else {
			return nil, fmt.Errorf("invalid network spec %+v", nw)
		}

		device := fmt.Sprintf("virtio-net,type=unixstream,path=%s,mac=%s", sock, mac)
		args = append(args, "--device", device)
	}

	if len(args) == 0 {
		return args, errors.New("no socket_vmnet networks defined")
	}

	return args, nil
}

func startUsernet(ctx context.Context, inst *limatype.Instance) (*usernet.Client, context.CancelFunc, error) {
	if firstUsernetIndex := limayaml.FirstUsernetIndex(inst.Config); firstUsernetIndex != -1 {
		return usernet.NewClientByName(inst.Config.Networks[firstUsernetIndex].Lima), nil, nil
	}
	// Start a in-process gvisor-tap-vsock
	endpointSock, err := usernet.SockWithDirectory(inst.Dir, "", usernet.EndpointSock)
	if err != nil {
		return nil, nil, err
	}
	krunkitSock, err := usernet.SockWithDirectory(inst.Dir, "", usernet.FDSock)
	if err != nil {
		return nil, nil, err
	}
	os.RemoveAll(endpointSock)
	os.RemoveAll(krunkitSock)
	ctx, cancel := context.WithCancel(ctx)
	err = usernet.StartGVisorNetstack(ctx, &usernet.GVisorNetstackOpts{
		MTU:      1500,
		Endpoint: endpointSock,
		FdSocket: krunkitSock,
		Async:    true,
		DefaultLeases: map[string]string{
			networks.SlirpIPAddress: limayaml.MACAddress(inst.Dir),
		},
		Subnet: networks.SlirpNetwork,
	})
	if err != nil {
		defer cancel()
		return nil, nil, err
	}
	subnetIP, _, err := net.ParseCIDR(networks.SlirpNetwork)
	return usernet.NewClient(endpointSock, subnetIP), cancel, err
}
