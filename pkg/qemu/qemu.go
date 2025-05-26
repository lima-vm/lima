// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/digitalocean/go-qemu/qmp"
	"github.com/digitalocean/go-qemu/qmp/raw"
	"github.com/docker/go-units"
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/fileutils"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/networks/usernet"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/filenames"
)

type Config struct {
	Name         string
	InstanceDir  string
	LimaYAML     *limayaml.LimaYAML
	SSHLocalPort int
	SSHAddress   string
	VirtioGA     bool
}

// minimumQemuVersion returns hardMin and softMin.
//
// hardMin is the hard minimum version of QEMU.
// The driver immediately returns the error when QEMU is older than this version.
//
// softMin is the oldest recommended version of QEMU.
// softMin must be >= hardMin.
//
// When updating this function, make sure to update
// `website/content/en/docs/config/vmtype.md` too.
func minimumQemuVersion() (hardMin, softMin semver.Version) {
	var h, s string
	switch runtime.GOOS {
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			// https://gitlab.com/qemu-project/qemu/-/issues/1990
			h, s = "8.2.1", "8.2.1"
		default:
			// The code specific to QEMU < 7.0 on macOS (https://github.com/lima-vm/lima/pull/703)
			// was removed in https://github.com/lima-vm/lima/pull/3491
			h, s = "7.0.0", "8.2.1"
		}
	default:
		// hardMin: Untested and maybe does not even work.
		// softMin: Ubuntu 22.04's QEMU. The oldest version that can be easily tested on GitHub Actions.
		h, s = "4.0.0", "6.2.0"
	}
	hardMin, softMin = *semver.New(h), *semver.New(s)
	if softMin.LessThan(hardMin) {
		// NOTREACHED
		logrus.Fatalf("internal error: QEMU: soft minimum version %v must be >= hard minimum version %v",
			softMin, hardMin)
	}
	return hardMin, softMin
}

// EnsureDisk also ensures the kernel and the initrd.
func EnsureDisk(ctx context.Context, cfg Config) error {
	diffDisk := filepath.Join(cfg.InstanceDir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	baseDisk := filepath.Join(cfg.InstanceDir, filenames.BaseDisk)
	kernel := filepath.Join(cfg.InstanceDir, filenames.Kernel)
	kernelCmdline := filepath.Join(cfg.InstanceDir, filenames.KernelCmdline)
	initrd := filepath.Join(cfg.InstanceDir, filenames.Initrd)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		var ensuredBaseDisk bool
		errs := make([]error, len(cfg.LimaYAML.Images))
		for i, f := range cfg.LimaYAML.Images {
			if _, err := fileutils.DownloadFile(ctx, baseDisk, f.File, true, "the image", *cfg.LimaYAML.Arch); err != nil {
				errs[i] = err
				continue
			}
			if f.Kernel != nil {
				if _, err := fileutils.DownloadFile(ctx, kernel, f.Kernel.File, false, "the kernel", *cfg.LimaYAML.Arch); err != nil {
					errs[i] = err
					continue
				}
				if f.Kernel.Cmdline != "" {
					if err := os.WriteFile(kernelCmdline, []byte(f.Kernel.Cmdline), 0o644); err != nil {
						errs[i] = err
						continue
					}
				}
			}
			if f.Initrd != nil {
				if _, err := fileutils.DownloadFile(ctx, initrd, *f.Initrd, false, "the initrd", *cfg.LimaYAML.Arch); err != nil {
					errs[i] = err
					continue
				}
			}
			ensuredBaseDisk = true
			break
		}
		if !ensuredBaseDisk {
			return fileutils.Errors(errs)
		}
	}
	diskSize, _ := units.RAMInBytes(*cfg.LimaYAML.Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	baseDiskInfo, err := imgutil.GetInfo(baseDisk)
	if err != nil {
		return fmt.Errorf("failed to get the information of base disk %q: %w", baseDisk, err)
	}
	if err = imgutil.AcceptableAsBasedisk(baseDiskInfo); err != nil {
		return fmt.Errorf("file %q is not acceptable as the base disk: %w", baseDisk, err)
	}
	if baseDiskInfo.Format == "" {
		return fmt.Errorf("failed to inspect the format of %q", baseDisk)
	}
	args := []string{"create", "-f", "qcow2"}
	if !isBaseDiskISO {
		args = append(args, "-F", baseDiskInfo.Format, "-b", baseDisk)
	}
	args = append(args, diffDisk, strconv.Itoa(int(diskSize)))
	cmd := exec.Command("qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
}

func newQmpClient(cfg Config) (*qmp.SocketMonitor, error) {
	qmpSock := filepath.Join(cfg.InstanceDir, filenames.QMPSock)
	qmpClient, err := qmp.NewSocketMonitor("unix", qmpSock, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return qmpClient, nil
}

func sendHmpCommand(cfg Config, cmd, tag string) (string, error) {
	qmpClient, err := newQmpClient(cfg)
	if err != nil {
		return "", err
	}
	if err := qmpClient.Connect(); err != nil {
		return "", err
	}
	defer func() { _ = qmpClient.Disconnect() }()
	rawClient := raw.NewMonitor(qmpClient)
	logrus.Infof("Sending HMP %s command", cmd)
	hmc := fmt.Sprintf("%s %s", cmd, tag)
	return rawClient.HumanMonitorCommand(hmc, nil)
}

func execImgCommand(cfg Config, args ...string) (string, error) {
	diffDisk := filepath.Join(cfg.InstanceDir, filenames.DiffDisk)
	args = append(args, diffDisk)
	logrus.Debugf("Running qemu-img %v command", args)
	cmd := exec.Command("qemu-img", args...)
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(b), err
}

func Del(cfg Config, run bool, tag string) error {
	if run {
		out, err := sendHmpCommand(cfg, "delvm", tag)
		// there can still be output, even if no error!
		if out != "" {
			logrus.Warnf("output: %s", strings.TrimSpace(out))
		}
		return err
	}
	// -d  deletes a snapshot
	_, err := execImgCommand(cfg, "snapshot", "-d", tag)
	return err
}

func Save(cfg Config, run bool, tag string) error {
	if run {
		out, err := sendHmpCommand(cfg, "savevm", tag)
		// there can still be output, even if no error!
		if out != "" {
			logrus.Warnf("output: %s", strings.TrimSpace(out))
		}
		return err
	}
	// -c  creates a snapshot
	_, err := execImgCommand(cfg, "snapshot", "-c", tag)
	return err
}

func Load(cfg Config, run bool, tag string) error {
	if run {
		out, err := sendHmpCommand(cfg, "loadvm", tag)
		// there can still be output, even if no error!
		if out != "" {
			logrus.Warnf("output: %s", strings.TrimSpace(out))
		}
		return err
	}
	// -a  applies a snapshot
	_, err := execImgCommand(cfg, "snapshot", "-a", tag)
	return err
}

// List returns a space-separated list of all snapshots, with header and newlines.
func List(cfg Config, run bool) (string, error) {
	if run {
		out, err := sendHmpCommand(cfg, "info", "snapshots")
		if err == nil {
			out = strings.ReplaceAll(out, "\r", "")
			out = strings.Replace(out, "List of snapshots present on all disks:\n", "", 1)
			out = strings.Replace(out, "There is no snapshot available.\n", "", 1)
		}
		return out, err
	}
	// -l  lists all snapshots
	args := []string{"snapshot", "-l"}
	out, err := execImgCommand(cfg, args...)
	if err == nil {
		// remove the redundant heading, result is not machine-parseable
		out = strings.Replace(out, "Snapshot list:\n", "", 1)
	}
	return out, err
}

func argValue(args []string, key string) (string, bool) {
	if !strings.HasPrefix(key, "-") {
		panic(fmt.Errorf("got unexpected key %q", key))
	}
	for i, s := range args {
		if s == key {
			if i == len(args)-1 {
				return "", true
			}
			value := args[i+1]
			if strings.HasPrefix(value, "-") {
				return "", true
			}
			return value, true
		}
	}
	return "", false
}

// appendArgsIfNoConflict can be used for: -cpu, -machine, -m, -boot ...
// appendArgsIfNoConflict cannot be used for: -drive, -cdrom, ...
func appendArgsIfNoConflict(args []string, k, v string) []string {
	if !strings.HasPrefix(k, "-") {
		panic(fmt.Errorf("got unexpected key %q", k))
	}
	switch k {
	case "-drive", "-cdrom", "-chardev", "-blockdev", "-netdev", "-device":
		panic(fmt.Errorf("appendArgsIfNoConflict() must not be called with k=%q", k))
	}

	if v == "" {
		if _, ok := argValue(args, k); ok {
			return args
		}
		return append(args, k)
	}

	if origV, ok := argValue(args, k); ok {
		logrus.Warnf("Not adding QEMU argument %q %q, as it conflicts with %q %q", k, v, k, origV)
		return args
	}
	return append(args, k, v)
}

type features struct {
	// AccelHelp is the output of `qemu-system-x86_64 -accel help`
	// e.g. "Accelerators supported in QEMU binary:\ntcg\nhax\nhvf\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	AccelHelp []byte
	// NetdevHelp is the output of `qemu-system-x86_64 -netdev help`
	// e.g. "Available netdev backend types:\nsocket\nhubport\ntap\nuser\nvde\nbridge\vhost-user\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	NetdevHelp []byte
	// MachineHelp is the output of `qemu-system-x86_64 -machine help`
	// e.g. "Supported machines are:\nakita...\n...virt-6.2...\n...virt-7.0...\n...\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	MachineHelp []byte
	// CPUHelp is the output of `qemu-system-x86_64 -cpu help`
	// e.g. "Available CPUs:\n...\nx86 base...\nx86 host...\n...\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	CPUHelp []byte
}

func inspectFeatures(exe, machine string) (*features, error) {
	var (
		f      features
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	cmd := exec.Command(exe, "-M", "none", "-accel", "help")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	}
	f.AccelHelp = stdout.Bytes()
	// on older versions qemu will write "help" output to stderr
	if len(f.AccelHelp) == 0 {
		f.AccelHelp = stderr.Bytes()
	}

	cmd = exec.Command(exe, "-M", "none", "-netdev", "help")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.Warnf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	} else {
		f.NetdevHelp = stdout.Bytes()
		if len(f.NetdevHelp) == 0 {
			f.NetdevHelp = stderr.Bytes()
		}
	}

	cmd = exec.Command(exe, "-machine", "help")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.Warnf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	} else {
		f.MachineHelp = stdout.Bytes()
		if len(f.MachineHelp) == 0 {
			f.MachineHelp = stderr.Bytes()
		}
	}

	// Avoid error: "No machine specified, and there is no default"
	cmd = exec.Command(exe, "-cpu", "help", "-machine", machine)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.Warnf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	} else {
		f.CPUHelp = stdout.Bytes()
		if len(f.CPUHelp) == 0 {
			f.CPUHelp = stderr.Bytes()
		}
	}

	return &f, nil
}

// adjustMemBytesDarwinARM64HVF adjusts the memory to be <= 3 GiB, only when the following conditions are met:
//
// - Host OS   <  macOS 12.4
// - Host Arch == arm64
// - Accel     == hvf
//
// This adjustment is required for avoiding host kernel panic. The issue was fixed in macOS 12.4 Beta 1.
// See https://github.com/lima-vm/lima/issues/795 https://gitlab.com/qemu-project/qemu/-/issues/903#note_911000975
func adjustMemBytesDarwinARM64HVF(memBytes int64, accel string) int64 {
	const safeSize = 3 * 1024 * 1024 * 1024 // 3 GiB
	if memBytes <= safeSize {
		return memBytes
	}
	if runtime.GOOS != "darwin" {
		return memBytes
	}
	if runtime.GOARCH != "arm64" {
		return memBytes
	}
	if accel != "hvf" {
		return memBytes
	}
	macOSProductVersion, err := osutil.ProductVersion()
	if err != nil {
		logrus.Warn(err)
		return memBytes
	}
	if !macOSProductVersion.LessThan(*semver.New("12.4.0")) {
		return memBytes
	}
	logrus.Warnf("Reducing the guest memory from %s to %s, to avoid host kernel panic on macOS <= 12.3; "+
		"Please update macOS to 12.4 or later; "+
		"See https://github.com/lima-vm/lima/issues/795 for the further background.",
		units.BytesSize(float64(memBytes)), units.BytesSize(float64(safeSize)))
	memBytes = safeSize
	return memBytes
}

// qemuMachine returns string to use for -machine.
func qemuMachine(arch limayaml.Arch) string {
	if arch == limayaml.X8664 {
		return "q35"
	}
	return "virt"
}

// audioDevice returns the default audio device.
func audioDevice() string {
	switch runtime.GOOS {
	case "darwin":
		return "coreaudio"
	case "linux":
		return "pa" // pulseaudio
	case "windows":
		return "dsound"
	}
	return "oss"
}

func Cmdline(ctx context.Context, cfg Config) (exe string, args []string, err error) {
	y := cfg.LimaYAML
	exe, args, err = Exe(*y.Arch)
	if err != nil {
		return "", nil, err
	}

	features, err := inspectFeatures(exe, qemuMachine(*y.Arch))
	if err != nil {
		return "", nil, err
	}

	version, err := getQemuVersion(exe)
	if err != nil {
		logrus.WithError(err).Warning("Failed to detect QEMU version")
	} else {
		logrus.Debugf("QEMU version %s detected", version.String())
		hardMin, softMin := minimumQemuVersion()
		if version.LessThan(hardMin) {
			logrus.Fatalf("QEMU %v is too old, %v or later required", version, hardMin)
		}
		if version.LessThan(softMin) {
			logrus.Warnf("QEMU %v is too old, %v or later is recommended", version, softMin)
		}
		if y.VMOpts.QEMU.MinimumVersion != nil && version.LessThan(*semver.New(*y.VMOpts.QEMU.MinimumVersion)) {
			logrus.Fatalf("QEMU %v is too old, template requires %q or later", version, *y.VMOpts.QEMU.MinimumVersion)
		}
	}

	// Architecture
	accel := Accel(*y.Arch)
	if !strings.Contains(string(features.AccelHelp), accel) {
		return "", nil, fmt.Errorf("accelerator %q is not supported by %s", accel, exe)
	}

	// Memory
	memBytes, err := units.RAMInBytes(*y.Memory)
	if err != nil {
		return "", nil, err
	}
	memBytes = adjustMemBytesDarwinARM64HVF(memBytes, accel)
	args = appendArgsIfNoConflict(args, "-m", strconv.Itoa(int(memBytes>>20)))

	if *y.MountType == limayaml.VIRTIOFS {
		args = appendArgsIfNoConflict(args, "-object",
			fmt.Sprintf("memory-backend-file,id=virtiofs-shm,size=%s,mem-path=/dev/shm,share=on", strconv.Itoa(int(memBytes))))
		args = appendArgsIfNoConflict(args, "-numa", "node,memdev=virtiofs-shm")
	}

	// CPU
	cpu := y.CPUType[*y.Arch]
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		switch {
		case strings.HasPrefix(cpu, "host"), strings.HasPrefix(cpu, "max"):
			if !strings.Contains(cpu, ",-pdpe1gb") {
				logrus.Warnf("On Intel Mac, CPU type %q typically needs \",-pdpe1gb\" option (https://stackoverflow.com/a/72863744/5167443)", cpu)
			}
		}
	}
	// `qemu-system-ppc64 -help` does not show "max", but it is actually accepted
	if cpu != "max" && !strings.Contains(string(features.CPUHelp), strings.Split(cpu, ",")[0]) {
		return "", nil, fmt.Errorf("cpu %q is not supported by %s", cpu, exe)
	}
	args = appendArgsIfNoConflict(args, "-cpu", cpu)

	// Machine
	switch *y.Arch {
	case limayaml.X8664:
		switch accel {
		case "tcg":
			// use q35 machine with vmware io port disabled.
			args = appendArgsIfNoConflict(args, "-machine", "q35,vmport=off")
			// use tcg accelerator with multi threading with 512MB translation block size
			// https://qemu-project.gitlab.io/qemu/devel/multi-thread-tcg.html?highlight=tcg
			// https://qemu-project.gitlab.io/qemu/system/invocation.html?highlight=tcg%20opts
			// this will make sure each vCPU will be backed by 1 host user thread.
			args = appendArgsIfNoConflict(args, "-accel", "tcg,thread=multi,tb-size=512")
			// This will disable CPU S3/S4 state.
			args = append(args, "-global", "ICH9-LPC.disable_s3=1")
			args = append(args, "-global", "ICH9-LPC.disable_s4=1")
		case "whpx":
			// whpx: injection failed, MSI (0, 0) delivery: 0, dest_mode: 0, trigger mode: 0, vector: 0
			args = appendArgsIfNoConflict(args, "-machine", "q35,accel="+accel+",kernel-irqchip=off")
		default:
			args = appendArgsIfNoConflict(args, "-machine", "q35,accel="+accel)
		}
	case limayaml.AARCH64:
		machine := "virt,accel=" + accel
		args = appendArgsIfNoConflict(args, "-machine", machine)
	case limayaml.RISCV64:
		// https://github.com/tianocore/edk2/blob/edk2-stable202408/OvmfPkg/RiscVVirt/README.md#test
		// > Note: the `acpi=off` machine property is specified because Linux guest
		// > support for ACPI (that is, the ACPI consumer side) is a work in progress.
		// > Currently, `acpi=off` is recommended unless you are developing ACPI support
		// > yourself.
		machine := "virt,acpi=off,accel=" + accel
		args = appendArgsIfNoConflict(args, "-machine", machine)
	case limayaml.ARMV7L:
		machine := "virt,accel=" + accel
		args = appendArgsIfNoConflict(args, "-machine", machine)
	case limayaml.PPC64LE:
		machine := "pseries,accel=" + accel
		args = appendArgsIfNoConflict(args, "-machine", machine)
	case limayaml.S390X:
		machine := "s390-ccw-virtio,accel=" + accel
		args = appendArgsIfNoConflict(args, "-machine", machine)
	}

	// SMP
	args = appendArgsIfNoConflict(args, "-smp",
		fmt.Sprintf("%d,sockets=1,cores=%d,threads=1", *y.CPUs, *y.CPUs))

	// Firmware
	legacyBIOS := *y.Firmware.LegacyBIOS
	if legacyBIOS && *y.Arch != limayaml.X8664 && *y.Arch != limayaml.ARMV7L {
		logrus.Warnf("field `firmware.legacyBIOS` is not supported for architecture %q, ignoring", *y.Arch)
		legacyBIOS = false
	}
	noFirmware := *y.Arch == limayaml.PPC64LE || *y.Arch == limayaml.S390X || legacyBIOS
	if !noFirmware {
		var firmware string
		firmwareInBios := runtime.GOOS == "windows"
		if envVar := os.Getenv("_LIMA_QEMU_UEFI_IN_BIOS"); envVar != "" {
			b, err := strconv.ParseBool(os.Getenv("_LIMA_QEMU_UEFI_IN_BIOS"))
			if err != nil {
				logrus.WithError(err).Warnf("invalid _LIMA_QEMU_UEFI_IN_BIOS value %q", envVar)
			} else {
				firmwareInBios = b
			}
		}
		firmwareInBios = firmwareInBios && *y.Arch == limayaml.X8664
		downloadedFirmware := filepath.Join(cfg.InstanceDir, filenames.QemuEfiCodeFD)
		firmwareWithVars := filepath.Join(cfg.InstanceDir, filenames.QemuEfiFullFD)
		if firmwareInBios {
			if _, stErr := os.Stat(firmwareWithVars); stErr == nil {
				firmware = firmwareWithVars
				logrus.Infof("Using existing firmware (%q)", firmware)
			}
		} else {
			if _, stErr := os.Stat(downloadedFirmware); errors.Is(stErr, os.ErrNotExist) {
			loop:
				for _, f := range y.Firmware.Images {
					switch f.VMType {
					case "", limayaml.QEMU:
						if f.Arch == *y.Arch {
							if _, err = fileutils.DownloadFile(ctx, downloadedFirmware, f.File, true, "UEFI code "+f.Location, *y.Arch); err != nil {
								logrus.WithError(err).Warnf("failed to download %q", f.Location)
								continue loop
							}
							firmware = downloadedFirmware
							logrus.Infof("Using firmware %q (downloaded from %q)", firmware, f.Location)
							break loop
						}
					}
				}
			} else {
				firmware = downloadedFirmware
				logrus.Infof("Using existing firmware (%q)", firmware)
			}
		}
		if firmware == "" {
			firmware, err = getFirmware(exe, *y.Arch)
			if err != nil {
				return "", nil, err
			}
			logrus.Infof("Using system firmware (%q)", firmware)
			if firmwareInBios {
				firmwareVars, err := getFirmwareVars(exe, *y.Arch)
				if err != nil {
					return "", nil, err
				}
				logrus.Infof("Using system firmware vars (%q)", firmwareVars)
				varsFile, err := os.Open(firmwareVars)
				if err != nil {
					return "", nil, err
				}
				defer varsFile.Close()
				codeFile, err := os.Open(firmware)
				if err != nil {
					return "", nil, err
				}
				defer codeFile.Close()
				resultFile, err := os.OpenFile(firmwareWithVars, os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					return "", nil, err
				}
				defer resultFile.Close()
				_, err = io.Copy(resultFile, varsFile)
				if err != nil {
					return "", nil, err
				}
				_, err = io.Copy(resultFile, codeFile)
				if err != nil {
					return "", nil, err
				}
				firmware = firmwareWithVars
			}
		}
		if firmware != "" {
			if firmwareInBios {
				args = append(args, "-bios", firmware)
			} else {
				args = append(args, "-drive", fmt.Sprintf("if=pflash,format=raw,readonly=on,file=%s", firmware))
			}
		}
	}

	// Disk
	baseDisk := filepath.Join(cfg.InstanceDir, filenames.BaseDisk)
	diffDisk := filepath.Join(cfg.InstanceDir, filenames.DiffDisk)
	extraDisks := []string{}
	for _, d := range y.AdditionalDisks {
		diskName := d.Name
		disk, err := store.InspectDisk(diskName)
		if err != nil {
			logrus.Errorf("could not load disk %q: %q", diskName, err)
			return "", nil, err
		}

		if disk.Instance != "" {
			if disk.InstanceDir != cfg.InstanceDir {
				logrus.Errorf("could not attach disk %q, in use by instance %q", diskName, disk.Instance)
				return "", nil, err
			}
			err = disk.Unlock()
			if err != nil {
				logrus.Errorf("could not unlock disk %q to reuse in the same instance %q", diskName, cfg.Name)
				return "", nil, err
			}
		}
		logrus.Infof("Mounting disk %q on %q", diskName, disk.MountPoint)
		err = disk.Lock(cfg.InstanceDir)
		if err != nil {
			logrus.Errorf("could not lock disk %q: %q", diskName, err)
			return "", nil, err
		}
		dataDisk := filepath.Join(disk.Dir, filenames.DataDisk)
		extraDisks = append(extraDisks, dataDisk)
	}

	isBaseDiskCDROM, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return "", nil, err
	}
	if isBaseDiskCDROM {
		args = appendArgsIfNoConflict(args, "-boot", "order=d,splash-time=0,menu=on")
		args = append(args, "-drive", fmt.Sprintf("file=%s,format=raw,media=cdrom,readonly=on", baseDisk))
	} else {
		args = appendArgsIfNoConflict(args, "-boot", "order=c,splash-time=0,menu=on")
	}
	if diskSize, _ := units.RAMInBytes(*cfg.LimaYAML.Disk); diskSize > 0 {
		args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio,discard=on", diffDisk))
	} else if !isBaseDiskCDROM {
		baseDiskInfo, err := imgutil.GetInfo(baseDisk)
		if err != nil {
			return "", nil, fmt.Errorf("failed to get the information of %q: %w", baseDisk, err)
		}
		if err = imgutil.AcceptableAsBasedisk(baseDiskInfo); err != nil {
			return "", nil, fmt.Errorf("file %q is not acceptable as the base disk: %w", baseDisk, err)
		}
		if baseDiskInfo.Format == "" {
			return "", nil, fmt.Errorf("failed to inspect the format of %q", baseDisk)
		}
		args = append(args, "-drive", fmt.Sprintf("file=%s,format=%s,if=virtio,discard=on", baseDisk, baseDiskInfo.Format))
	}
	for _, extraDisk := range extraDisks {
		args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio,discard=on", extraDisk))
	}

	// cloud-init
	args = append(args,
		"-drive", "id=cdrom0,if=none,format=raw,readonly=on,file="+filepath.Join(cfg.InstanceDir, filenames.CIDataISO),
		"-device", "virtio-scsi-pci,id=scsi0",
		"-device", "scsi-cd,bus=scsi0.0,drive=cdrom0")

	// Kernel
	kernel := filepath.Join(cfg.InstanceDir, filenames.Kernel)
	kernelCmdline := filepath.Join(cfg.InstanceDir, filenames.KernelCmdline)
	initrd := filepath.Join(cfg.InstanceDir, filenames.Initrd)
	if _, err := os.Stat(kernel); err == nil {
		args = appendArgsIfNoConflict(args, "-kernel", kernel)
	}
	if b, err := os.ReadFile(kernelCmdline); err == nil {
		args = appendArgsIfNoConflict(args, "-append", string(b))
	}
	if _, err := os.Stat(initrd); err == nil {
		args = appendArgsIfNoConflict(args, "-initrd", initrd)
	}

	// Network
	// Configure default usernetwork with limayaml.MACAddress(driver.Instance.Dir) for eth0 interface
	firstUsernetIndex := limayaml.FirstUsernetIndex(y)
	if firstUsernetIndex == -1 {
		args = append(args, "-netdev", fmt.Sprintf("user,id=net0,net=%s,dhcpstart=%s,hostfwd=tcp:%s:%d-:22",
			networks.SlirpNetwork, networks.SlirpIPAddress, cfg.SSHAddress, cfg.SSHLocalPort))
	} else {
		qemuSock, err := usernet.Sock(y.Networks[firstUsernetIndex].Lima, usernet.QEMUSock)
		if err != nil {
			return "", nil, err
		}
		args = append(args, "-netdev", fmt.Sprintf("socket,id=net0,fd={{ fd_connect %q }}", qemuSock))
	}
	virtioNet := "virtio-net-pci"
	if *y.Arch == limayaml.S390X {
		// virtio-net-pci does not work on EL, while it works on Ubuntu
		// https://github.com/lima-vm/lima/pull/3319/files#r1986388345
		virtioNet = "virtio-net-ccw"
	}
	args = append(args, "-device", virtioNet+",netdev=net0,mac="+limayaml.MACAddress(cfg.InstanceDir))

	for i, nw := range y.Networks {
		if nw.Lima != "" {
			nwCfg, err := networks.LoadConfig()
			if err != nil {
				return "", nil, err
			}

			// Handle usernet connections
			isUsernet, err := nwCfg.Usernet(nw.Lima)
			if err != nil {
				return "", nil, err
			}
			if isUsernet {
				if i == firstUsernetIndex {
					continue
				}
				qemuSock, err := usernet.Sock(nw.Lima, usernet.QEMUSock)
				if err != nil {
					return "", nil, err
				}
				args = append(args, "-netdev", fmt.Sprintf("socket,id=net%d,fd={{ fd_connect %q }}", i+1, qemuSock))
				args = append(args, "-device", fmt.Sprintf("%s,netdev=net%d,mac=%s", virtioNet, i+1, nw.MACAddress))
			} else {
				if runtime.GOOS != "darwin" {
					return "", nil, fmt.Errorf("networks.yaml '%s' configuration is only supported on macOS right now", nw.Lima)
				}
				logrus.Debugf("Using socketVMNet (%q)", nwCfg.Paths.SocketVMNet)
				sock, err := networks.Sock(nw.Lima)
				if err != nil {
					return "", nil, err
				}
				args = append(args, "-netdev", fmt.Sprintf("socket,id=net%d,fd={{ fd_connect %q }}", i+1, sock))
				// TODO: should we also validate that the socket exists, or do we rely on the
				// networks reconciler to throw an error when the network cannot start?
			}
		} else if nw.Socket != "" {
			args = append(args, "-netdev", fmt.Sprintf("socket,id=net%d,fd={{ fd_connect %q }}", i+1, nw.Socket))
		} else {
			return "", nil, fmt.Errorf("invalid network spec %+v", nw)
		}
		args = append(args, "-device", fmt.Sprintf("%s,netdev=net%d,mac=%s", virtioNet, i+1, nw.MACAddress))
	}

	// virtio-rng-pci accelerates starting up the OS, according to https://wiki.gentoo.org/wiki/QEMU/Options
	args = append(args, "-device", "virtio-rng-pci")

	// Input
	input := "mouse"

	// Sound
	if *y.Audio.Device != "" {
		id := "default"
		// audio device
		audiodev := *y.Audio.Device
		if audiodev == "default" {
			audiodev = audioDevice()
		}
		audiodev += fmt.Sprintf(",id=%s", id)
		args = append(args, "-audiodev", audiodev)
		// audio controller
		args = append(args, "-device", "ich9-intel-hda")
		// audio codec
		args = append(args, "-device", fmt.Sprintf("hda-output,audiodev=%s", id))
	}
	// Graphics
	if *y.Video.Display != "" {
		display := *y.Video.Display
		if display == "vnc" {
			display += "=" + *y.Video.VNC.Display
			display += ",password=on"
			// use tablet to avoid double cursors
			input = "tablet"
		}
		args = appendArgsIfNoConflict(args, "-display", display)
	}

	switch *y.Arch {
	// FIXME: use virtio-gpu on all the architectures
	case limayaml.X8664, limayaml.RISCV64:
		args = append(args, "-device", "virtio-vga")
	default:
		args = append(args, "-device", "virtio-gpu")
	}
	args = append(args, "-device", "virtio-keyboard-pci")
	args = append(args, "-device", "virtio-"+input+"-pci")
	args = append(args, "-device", "qemu-xhci,id=usb-bus")

	// Parallel
	args = append(args, "-parallel", "none")

	// Serial (default)
	// This is ttyS0 for Intel and RISC-V, ttyAMA0 for ARM.
	serialSock := filepath.Join(cfg.InstanceDir, filenames.SerialSock)
	if err := os.RemoveAll(serialSock); err != nil {
		return "", nil, err
	}
	serialLog := filepath.Join(cfg.InstanceDir, filenames.SerialLog)
	if err := os.RemoveAll(serialLog); err != nil {
		return "", nil, err
	}
	const serialChardev = "char-serial"
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off,logfile=%s", serialChardev, serialSock, serialLog))
	args = append(args, "-serial", "chardev:"+serialChardev)

	// Serial (PCI, ARM only)
	// On ARM, the default serial is ttyAMA0, this PCI serial is ttyS0.
	// https://gitlab.com/qemu-project/qemu/-/issues/1801#note_1494720586
	switch *y.Arch {
	case limayaml.AARCH64, limayaml.ARMV7L:
		serialpSock := filepath.Join(cfg.InstanceDir, filenames.SerialPCISock)
		if err := os.RemoveAll(serialpSock); err != nil {
			return "", nil, err
		}
		serialpLog := filepath.Join(cfg.InstanceDir, filenames.SerialPCILog)
		if err := os.RemoveAll(serialpLog); err != nil {
			return "", nil, err
		}
		const serialpChardev = "char-serial-pci"
		args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off,logfile=%s", serialpChardev, serialpSock, serialpLog))
		args = append(args, "-device", "pci-serial,chardev="+serialpChardev)
	}

	// Serial (virtio)
	serialvSock := filepath.Join(cfg.InstanceDir, filenames.SerialVirtioSock)
	if err := os.RemoveAll(serialvSock); err != nil {
		return "", nil, err
	}
	serialvLog := filepath.Join(cfg.InstanceDir, filenames.SerialVirtioLog)
	if err := os.RemoveAll(serialvLog); err != nil {
		return "", nil, err
	}
	const serialvChardev = "char-serial-virtio"
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off,logfile=%s", serialvChardev, serialvSock, serialvLog))
	// max_ports=1 is required for https://github.com/lima-vm/lima/issues/1689 https://github.com/lima-vm/lima/issues/1691
	serialvMaxPorts := 1
	if *y.Arch == limayaml.S390X {
		serialvMaxPorts++ // needed to avoid `virtio-serial-bus: Out-of-range port id specified, max. allowed: 0`
	}
	args = append(args, "-device", fmt.Sprintf("virtio-serial-pci,id=virtio-serial0,max_ports=%d", serialvMaxPorts))
	args = append(args, "-device", fmt.Sprintf("virtconsole,chardev=%s,id=console0", serialvChardev))

	// We also want to enable vsock here, but QEMU does not support vsock for macOS hosts

	if *y.MountType == limayaml.NINEP || *y.MountType == limayaml.VIRTIOFS {
		for i, f := range y.Mounts {
			tag := fmt.Sprintf("mount%d", i)
			if err := os.MkdirAll(f.Location, 0o755); err != nil {
				return "", nil, err
			}

			switch *y.MountType {
			case limayaml.NINEP:
				options := "local"
				options += fmt.Sprintf(",mount_tag=%s", tag)
				options += fmt.Sprintf(",path=%s", f.Location)
				options += fmt.Sprintf(",security_model=%s", *f.NineP.SecurityModel)
				if !*f.Writable {
					options += ",readonly"
				}
				args = append(args, "-virtfs", options)
			case limayaml.VIRTIOFS:
				// Note that read-only mode is not supported on the QEMU/virtiofsd side yet:
				// https://gitlab.com/virtio-fs/virtiofsd/-/issues/97
				chardev := fmt.Sprintf("char-virtiofs-%d", i)
				vhostSock := filepath.Join(cfg.InstanceDir, fmt.Sprintf(filenames.VhostSock, i))
				args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s", chardev, vhostSock))

				options := "vhost-user-fs-pci"
				options += fmt.Sprintf(",queue-size=%d", *f.Virtiofs.QueueSize)
				options += fmt.Sprintf(",chardev=%s", chardev)
				options += fmt.Sprintf(",tag=%s", tag)
				args = append(args, "-device", options)
			}
		}
	}

	// QMP
	qmpSock := filepath.Join(cfg.InstanceDir, filenames.QMPSock)
	if err := os.RemoveAll(qmpSock); err != nil {
		return "", nil, err
	}
	const qmpChardev = "char-qmp"
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off", qmpChardev, qmpSock))
	args = append(args, "-qmp", "chardev:"+qmpChardev)

	if cfg.VirtioGA {
		// Guest agent via serialport
		guestSock := filepath.Join(cfg.InstanceDir, filenames.GuestAgentSock)
		args = append(args, "-chardev", fmt.Sprintf("socket,path=%s,server=on,wait=off,id=qga0", guestSock))
		args = append(args, "-device", "virtio-serial")
		args = append(args, "-device", "virtserialport,chardev=qga0,name="+filenames.VirtioPort)
	}

	// QEMU process
	args = append(args, "-name", "lima-"+cfg.Name)
	args = append(args, "-pidfile", filepath.Join(cfg.InstanceDir, filenames.PIDFile(*y.VMType)))

	return exe, args, nil
}

func FindVirtiofsd(qemuExe string) (string, error) {
	type vhostUserBackend struct {
		BackendType string `json:"type"`
		Binary      string `json:"binary"`
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	const relativePath = "share/qemu/vhost-user"

	binDir := filepath.Dir(qemuExe)                  // "/usr/local/bin"
	usrDir := filepath.Dir(binDir)                   // "/usr/local"
	userLocalDir := filepath.Join(homeDir, ".local") // "$HOME/.local"

	candidates := []string{
		filepath.Join(userLocalDir, relativePath),
		filepath.Join(usrDir, relativePath),
	}

	if usrDir != "/usr" {
		candidates = append(candidates, filepath.Join("/usr", relativePath))
	}

	for _, vhostCfgsDir := range candidates {
		logrus.Debugf("Checking vhost directory %s", vhostCfgsDir)

		cfgEntries, err := os.ReadDir(vhostCfgsDir)
		if err != nil {
			logrus.Debugf("Failed to list vhost directory: %v", err)
			continue
		}

		for _, cfgEntry := range cfgEntries {
			logrus.Debugf("Checking vhost vhostCfg %s", cfgEntry.Name())
			if !strings.HasSuffix(cfgEntry.Name(), ".json") {
				continue
			}

			var vhostCfg vhostUserBackend
			contents, err := os.ReadFile(filepath.Join(vhostCfgsDir, cfgEntry.Name()))
			if err == nil {
				err = json.Unmarshal(contents, &vhostCfg)
			}

			if err != nil {
				logrus.Warnf("Failed to load vhost-user config %s: %v", cfgEntry.Name(), err)
				continue
			}
			logrus.Debugf("%v", vhostCfg)

			if vhostCfg.BackendType != "fs" {
				continue
			}

			// Only rust virtiofsd supports --version, so use that to make sure this isn't
			// QEMU's virtiofsd, which requires running as root.
			cmd := exec.Command(vhostCfg.Binary, "--version")
			output, err := cmd.CombinedOutput()
			if err != nil {
				logrus.Warnf("Failed to run %s --version (is this QEMU virtiofsd?): %s: %s",
					vhostCfg.Binary, err, output)
				continue
			}

			return vhostCfg.Binary, nil
		}
	}

	return "", errors.New("failed to locate virtiofsd")
}

func VirtiofsdCmdline(cfg Config, mountIndex int) ([]string, error) {
	mount := cfg.LimaYAML.Mounts[mountIndex]

	vhostSock := filepath.Join(cfg.InstanceDir, fmt.Sprintf(filenames.VhostSock, mountIndex))
	// qemu_driver has to wait for the socket to appear, so make sure any old ones are removed here.
	if err := os.Remove(vhostSock); err != nil && !errors.Is(err, fs.ErrNotExist) {
		logrus.Warnf("Failed to remove old vhost socket: %v", err)
	}

	return []string{
		"--socket-path", vhostSock,
		"--shared-dir", mount.Location,
	}, nil
}

// qemuArch returns the arch string used by qemu.
func qemuArch(arch limayaml.Arch) string {
	switch arch {
	case limayaml.ARMV7L:
		return "arm"
	case limayaml.PPC64LE:
		return "ppc64"
	default:
		return arch
	}
}

// qemuEdk2 returns the arch string used by `/usr/local/share/qemu/edk2-*-code.fd`.
func qemuEdk2Arch(arch limayaml.Arch) string {
	if arch == limayaml.RISCV64 {
		return "riscv"
	}
	return qemuArch(arch)
}

func Exe(arch limayaml.Arch) (exe string, args []string, err error) {
	exeBase := "qemu-system-" + qemuArch(arch)
	envK := "QEMU_SYSTEM_" + strings.ToUpper(qemuArch(arch))
	if envV := os.Getenv(envK); envV != "" {
		ss, err := shellwords.Parse(envV)
		if err != nil {
			return "", nil, fmt.Errorf("failed to parse %s value %q: %w", envK, envV, err)
		}
		exeBase, args = ss[0], ss[1:]
		if len(args) != 0 {
			logrus.Warnf("Specifying args (%v) via $%s is supported only for debugging!", args, envK)
		}
	}
	exe, err = exec.LookPath(exeBase)
	if err != nil {
		return "", nil, err
	}
	return exe, args, nil
}

func Accel(arch limayaml.Arch) string {
	if limayaml.IsNativeArch(arch) {
		switch runtime.GOOS {
		case "darwin":
			return "hvf"
		case "linux":
			return "kvm"
		case "netbsd":
			return "nvmm"
		case "dragonfly":
			return "nvmm"
		case "windows":
			return "whpx"
		}
	}
	return "tcg"
}

func parseQemuVersion(output string) (*semver.Version, error) {
	lines := strings.Split(output, "\n")
	regex := regexp.MustCompile(`^QEMU emulator version (\d+\.\d+\.\d+)`)
	matches := regex.FindStringSubmatch(lines[0])
	if len(matches) == 2 {
		return semver.New(matches[1]), nil
	}
	return &semver.Version{}, fmt.Errorf("failed to parse %v", output)
}

func getQemuVersion(qemuExe string) (*semver.Version, error) {
	var (
		stdout bytes.Buffer
		stderr bytes.Buffer
	)
	cmd := exec.Command(qemuExe, "--version")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run %v: stdout=%q, stderr=%q", cmd.Args, stdout.String(), stderr.String())
	}

	return parseQemuVersion(stdout.String())
}

func getFirmware(qemuExe string, arch limayaml.Arch) (string, error) {
	switch arch {
	case limayaml.X8664, limayaml.AARCH64, limayaml.ARMV7L, limayaml.RISCV64:
	default:
		return "", fmt.Errorf("unexpected architecture: %q", arch)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	binDir := filepath.Dir(qemuExe)                  // "/usr/local/bin"
	localDir := filepath.Dir(binDir)                 // "/usr/local"
	userLocalDir := filepath.Join(homeDir, ".local") // "$HOME/.local"

	relativePath := fmt.Sprintf("share/qemu/edk2-%s-code.fd", qemuEdk2Arch(arch))
	relativePathWin := fmt.Sprintf("share/edk2-%s-code.fd", qemuEdk2Arch(arch))
	candidates := []string{
		filepath.Join(userLocalDir, relativePath), // XDG-like
		filepath.Join(localDir, relativePath),     // macOS (homebrew)
		filepath.Join(binDir, relativePathWin),    // Windows installer
	}

	switch arch {
	case limayaml.X8664:
		// Debian package "ovmf"
		candidates = append(candidates, "/usr/share/OVMF/OVMF_CODE.fd")
		candidates = append(candidates, "/usr/share/OVMF/OVMF_CODE_4M.fd")
		// Fedora package "edk2-ovmf"
		candidates = append(candidates, "/usr/share/edk2/ovmf/OVMF_CODE.fd")
		// openSUSE package "qemu-ovmf-x86_64"
		candidates = append(candidates, "/usr/share/qemu/ovmf-x86_64.bin")
		// Archlinux package "edk2-ovmf"
		candidates = append(candidates, "/usr/share/edk2-ovmf/x64/OVMF_CODE.fd")
	case limayaml.AARCH64:
		// Debian package "qemu-efi-aarch64"
		// Fedora package "edk2-aarch64"
		candidates = append(candidates, "/usr/share/AAVMF/AAVMF_CODE.fd")
		// Debian package "qemu-efi-aarch64" (unpadded, backwards compatibility)
		candidates = append(candidates, "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd")
	case limayaml.ARMV7L:
		// Debian package "qemu-efi-arm"
		// Fedora package "edk2-arm"
		candidates = append(candidates, "/usr/share/AAVMF/AAVMF32_CODE.fd")
	case limayaml.RISCV64:
		// Debian package "qemu-efi-riscv64"
		candidates = append(candidates, "/usr/share/qemu-efi-riscv64/RISCV_VIRT_CODE.fd")
		// Fedora package "edk2-riscv64"
		candidates = append(candidates, "/usr/share/edk2/riscv/RISCV_VIRT_CODE.fd")
	}

	logrus.Debugf("firmware candidates = %v", candidates)

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}

	if arch == limayaml.X8664 {
		return "", fmt.Errorf("could not find firmware for %q (hint: try setting `firmware.legacyBIOS` to `true`)", arch)
	}
	qemuArch := strings.TrimPrefix(filepath.Base(qemuExe), "qemu-system-")
	return "", fmt.Errorf("could not find firmware for %q (hint: try copying the \"edk-%s-code.fd\" firmware to $HOME/.local/share/qemu/)", arch, qemuArch)
}

func getFirmwareVars(qemuExe string, arch limayaml.Arch) (string, error) {
	var targetArch string
	switch arch {
	case limayaml.X8664:
		targetArch = "i386" // vars are unified between i386 and x86_64 and normally only former is bundled
	default:
		return "", fmt.Errorf("unexpected architecture: %q", arch)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	binDir := filepath.Dir(qemuExe)                  // "/usr/local/bin"
	localDir := filepath.Dir(binDir)                 // "/usr/local"
	userLocalDir := filepath.Join(homeDir, ".local") // "$HOME/.local"

	relativePath := fmt.Sprintf("share/qemu/edk2-%s-vars.fd", qemuEdk2Arch(targetArch))
	relativePathWin := fmt.Sprintf("share/edk2-%s-vars.fd", qemuEdk2Arch(targetArch))
	candidates := []string{
		filepath.Join(userLocalDir, relativePath), // XDG-like
		filepath.Join(localDir, relativePath),     // macOS (homebrew)
		filepath.Join(binDir, relativePathWin),    // Windows installer
	}

	logrus.Debugf("firmware vars candidates = %v", candidates)

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}

	return "", fmt.Errorf("could not find firmware vars for %q", arch)
}
