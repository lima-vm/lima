// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/v2/pkg/imgutil/proxyimgutil"
	"github.com/lima-vm/lima/v2/pkg/iso9660util"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
	"github.com/lima-vm/lima/v2/pkg/limayaml"
	"github.com/lima-vm/lima/v2/pkg/networks"
)

const (
	restfulURI   = "krun-restful.sock" // This is for setting/checking the state of the VM
	logLevelInfo = "3"
	KrunEfi      = "krun-efi" // efi variable store
)

// Cmdline constructs the command line arguments for krunkit based on the instance configuration.
func Cmdline(inst *limatype.Instance) (*exec.Cmd, error) {
	var args = []string{
		"--memory", strconv.Itoa(2048),
		"--cpus", fmt.Sprintf("%d", *inst.Config.CPUs),
		"--device", fmt.Sprintf("virtio-serial,logFilePath=%s", filepath.Join(inst.Dir, filenames.SerialLog)),
		"--krun-log-level", logLevelInfo,
		"--restful-uri", fmt.Sprintf("unix://%s", restfulSocketPath(inst)),
		"--bootloader", fmt.Sprintf("efi,variable-store=%s,create", filepath.Join(inst.Dir, KrunEfi)),
		"--device", fmt.Sprintf("virtio-blk,path=%s,format=raw", filepath.Join(inst.Dir, filenames.DiffDisk)),
		"--device", fmt.Sprintf("virtio-blk,path=%s", filepath.Join(inst.Dir, filenames.CIDataISO)),
	}

	// TODO: socket_vmnet and ssh not working
	networkArgs, err := buildNetworkArgs(inst)
	if err != nil {
		return nil, fmt.Errorf("failed to build network arguments: %w", err)
	}
	args = append(args, networkArgs...)

	return exec.CommandContext(context.Background(), "krunkit", args...), nil
}

func buildNetworkArgs(inst *limatype.Instance) ([]string, error) {
	var args []string
	nwCfg, err := networks.LoadConfig()
	if err != nil {
		return nil, err
	}
	socketVMNetOk, err := nwCfg.IsDaemonInstalled(networks.SocketVMNet)
	if err != nil {
		return nil, err
	}
	if socketVMNetOk {
		sock, err := networks.Sock("shared")
		if err != nil {
			return nil, err
		}
		networkArg := fmt.Sprintf("virtio-net,type=unixstream,path=%s,mac=%s,offloading=true",
			sock,
			limayaml.MACAddress(inst.Dir),
		)
		args = append(args, "--device", networkArg)

		return args, nil
	}

	return args, errors.New("socket_vmnet is not installed")
}

func restfulSocketPath(inst *limatype.Instance) string {
	return filepath.Join(inst.Dir, restfulURI)
}

func EnsureDisk(ctx context.Context, inst *limatype.Instance) error {
	diffDisk := filepath.Join(inst.Dir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	diskUtil := proxyimgutil.NewDiskUtil(ctx)

	baseDisk := filepath.Join(inst.Dir, filenames.BaseDisk)

	diskSize, _ := units.RAMInBytes(*inst.Config.Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	if isBaseDiskISO {
		// Create an empty data volume (sparse)
		diffDiskF, err := os.Create(diffDisk)
		if err != nil {
			return err
		}

		err = diskUtil.MakeSparse(ctx, diffDiskF, 0)
		if err != nil {
			diffDiskF.Close()
			return fmt.Errorf("failed to create sparse diff disk %q: %w", diffDisk, err)
		}
		return diffDiskF.Close()
	}
	if err = diskUtil.ConvertToRaw(ctx, baseDisk, diffDisk, &diskSize, false); err != nil {
		return fmt.Errorf("failed to convert %q to a raw disk %q: %w", baseDisk, diffDisk, err)
	}
	return err
}
