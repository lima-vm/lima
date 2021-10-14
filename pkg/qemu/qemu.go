package qemu

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/downloader"
	"github.com/lima-vm/lima/pkg/iso9660util"
	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/lima/pkg/networks"
	qemu "github.com/lima-vm/lima/pkg/qemu/const"
	"github.com/lima-vm/lima/pkg/qemu/imgutil"
	"github.com/lima-vm/lima/pkg/store/filenames"
	"github.com/mattn/go-shellwords"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Name         string
	InstanceDir  string
	LimaYAML     *limayaml.LimaYAML
	SSHLocalPort int
}

func EnsureDisk(cfg Config) error {
	diffDisk := filepath.Join(cfg.InstanceDir, filenames.DiffDisk)
	if _, err := os.Stat(diffDisk); err == nil || !errors.Is(err, os.ErrNotExist) {
		// disk is already ensured
		return err
	}

	baseDisk := filepath.Join(cfg.InstanceDir, filenames.BaseDisk)
	if _, err := os.Stat(baseDisk); errors.Is(err, os.ErrNotExist) {
		var ensuredBaseDisk bool
		errs := make([]error, len(cfg.LimaYAML.Images))
		for i, f := range cfg.LimaYAML.Images {
			if f.Arch != cfg.LimaYAML.Arch {
				errs[i] = fmt.Errorf("unsupported arch: %q", f.Arch)
				continue
			}
			logrus.WithField("digest", f.Digest).Infof("Attempting to download the image from %q", f.Location)
			res, err := downloader.Download(baseDisk, f.Location,
				downloader.WithCache(),
				downloader.WithExpectedDigest(f.Digest),
			)
			if err != nil {
				errs[i] = fmt.Errorf("failed to download %q: %w", f.Location, err)
				continue
			}
			logrus.Debugf("res.ValidatedDigest=%v", res.ValidatedDigest)
			switch res.Status {
			case downloader.StatusDownloaded:
				logrus.Infof("Downloaded image from %q", f.Location)
			case downloader.StatusUsedCache:
				logrus.Infof("Using cache %q", res.CachePath)
			default:
				logrus.Warnf("Unexpected result from downloader.Download(): %+v", res)
			}
			ensuredBaseDisk = true
			break
		}
		if !ensuredBaseDisk {
			return fmt.Errorf("failed to download the image, attempted %d candidates, errors=%v",
				len(cfg.LimaYAML.Images), errs)
		}
	}
	diskSize, _ := units.RAMInBytes(cfg.LimaYAML.Disk)
	if diskSize == 0 {
		return nil
	}
	isBaseDiskISO, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return err
	}
	args := []string{"create", "-f", "qcow2"}
	if !isBaseDiskISO {
		baseDiskFormat, err := imgutil.DetectFormat(baseDisk)
		if err != nil {
			return err
		}
		args = append(args, "-F", baseDiskFormat, "-b", baseDisk)
	}
	args = append(args, diffDisk, strconv.Itoa(int(diskSize)))
	cmd := exec.Command("qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to run %v: %q: %w", cmd.Args, string(out), err)
	}
	return nil
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
	// NetdevHelp is the output of `qemu-system-x86_64 -accel help`
	// e.g. "Accelerators supported in QEMU binary:\ntcg\nhax\nhvf\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	AccelHelp []byte
	// NetdevHelp is the output of `qemu-system-x86_64 -netdev help`
	// e.g. "Available netdev backend types:\nsocket\nhubport\ntap\nuser\nvde\nbridge\vhost-user\n"
	// Not machine-readable, but checking strings.Contains() should be fine.
	NetdevHelp []byte
}

func inspectFeatures(exe string) (*features, error) {
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
	return &f, nil
}

func Cmdline(cfg Config) (string, []string, error) {
	y := cfg.LimaYAML
	exe, args, err := getExe(y.Arch)
	if err != nil {
		return "", nil, err
	}

	features, err := inspectFeatures(exe)
	if err != nil {
		return "", nil, err
	}

	// Architecture
	accel := getAccel(y.Arch)
	if !strings.Contains(string(features.AccelHelp), accel) {
		errStr := fmt.Sprintf("accelerator %q is not supported by %s", accel, exe)
		if accel == "hvf" && y.Arch == limayaml.AARCH64 {
			errStr += " ( Hint: as of August 2021, qemu-system-aarch64 on ARM Mac needs to be patched for enabling hvf accelerator,"
			errStr += " see https://gist.github.com/nrjdalal/e70249bb5d2e9d844cc203fd11f74c55 )"
		}
		return "", nil, errors.New(errStr)
	}
	switch y.Arch {
	case limayaml.X8664:
		cpu := "Haswell-v4"
		if isNativeArch(y.Arch) {
			cpu = "host"
		}
		args = appendArgsIfNoConflict(args, "-cpu", cpu)
		args = appendArgsIfNoConflict(args, "-machine", "q35,accel="+accel)
	case limayaml.AARCH64:
		cpu := "cortex-a72"
		if isNativeArch(y.Arch) {
			cpu = "host"
		}
		args = appendArgsIfNoConflict(args, "-cpu", cpu)
		args = appendArgsIfNoConflict(args, "-machine", "virt,accel="+accel+",highmem=off")
	}

	// SMP
	args = appendArgsIfNoConflict(args, "-smp",
		fmt.Sprintf("%d,sockets=1,cores=%d,threads=1", y.CPUs, y.CPUs))

	// Memory
	memBytes, err := units.RAMInBytes(y.Memory)
	if err != nil {
		return "", nil, err
	}
	args = appendArgsIfNoConflict(args, "-m", strconv.Itoa(int(memBytes>>20)))

	// Firmware
	legacyBIOS := y.Firmware.LegacyBIOS
	if legacyBIOS && y.Arch != limayaml.X8664 {
		logrus.Warnf("field `firmware.legacyBIOS` is not supported for architecture %q, ignoring", y.Arch)
		legacyBIOS = false
	}
	if !legacyBIOS {
		firmware, err := getFirmware(exe, y.Arch)
		if err != nil {
			return "", nil, err
		}
		args = append(args, "-drive", fmt.Sprintf("if=pflash,format=raw,readonly=on,file=%s", firmware))
	}

	baseDisk := filepath.Join(cfg.InstanceDir, filenames.BaseDisk)
	diffDisk := filepath.Join(cfg.InstanceDir, filenames.DiffDisk)
	isBaseDiskCDROM, err := iso9660util.IsISO9660(baseDisk)
	if err != nil {
		return "", nil, err
	}
	if isBaseDiskCDROM {
		args = appendArgsIfNoConflict(args, "-boot", "order=d,splash-time=0,menu=on")
		args = append(args, "-drive", fmt.Sprintf("file=%s,media=cdrom,readonly=on", baseDisk))
	} else {
		args = appendArgsIfNoConflict(args, "-boot", "order=c,splash-time=0,menu=on")
	}
	if diskSize, _ := units.RAMInBytes(cfg.LimaYAML.Disk); diskSize > 0 {
		args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio", diffDisk))
	} else if !isBaseDiskCDROM {
		args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio", baseDisk))
	}
	// cloud-init
	args = append(args, "-cdrom", filepath.Join(cfg.InstanceDir, filenames.CIDataISO))

	// Network
	args = append(args, "-netdev", fmt.Sprintf("user,id=net0,net=%s,dhcpstart=%s,hostfwd=tcp:127.0.0.1:%d-:22",
		qemu.SlirpNetwork, qemu.SlirpIPAddress, cfg.SSHLocalPort))
	args = append(args, "-device", "virtio-net-pci,netdev=net0,mac="+limayaml.MACAddress(cfg.InstanceDir))
	if len(y.Networks) > 0 && !strings.Contains(string(features.NetdevHelp), "vde") {
		return "", nil, fmt.Errorf("netdev \"vde\" is not supported by %s ( Hint: recompile QEMU with `configure --enable-vde` )", exe)
	}
	for i, nw := range y.Networks {
		var vdeSock string
		if nw.Lima != "" {
			vdeSock, err = networks.VDESock(nw.Lima)
			if err != nil {
				return "", nil, err
			}
			// TODO: should we also validate that the socket exists, or do we rely on the
			// networks reconciler to throw an error when the network cannot start?
		} else {
			// VDE4 accepts VNL like vde:///var/run/vde.ctl as well as file path like /var/run/vde.ctl .
			// VDE2 only accepts the latter form.
			// VDE2 supports macOS but VDE4 does not yet, so we trim vde:// prefix here for VDE2 compatibility.
			vdeSock = strings.TrimPrefix(nw.VNL, "vde://")
			if !strings.Contains(vdeSock, "://") {
				if _, err := os.Stat(vdeSock); err != nil {
					return "", nil, fmt.Errorf("cannot use VNL %q: %w", nw.VNL, err)
				}
				// vdeSock is a directory, unless vde.SwitchPort == 65535 (PTP)
				actualSocket := filepath.Join(vdeSock, "ctl")
				if nw.SwitchPort == 65535 { // PTP
					actualSocket = vdeSock
				}
				if st, err := os.Stat(actualSocket); err != nil {
					return "", nil, fmt.Errorf("cannot use VNL %q: failed to stat %q: %w", nw.VNL, actualSocket, err)
				} else if st.Mode()&fs.ModeSocket == 0 {
					return "", nil, fmt.Errorf("cannot use VNL %q: %q is not a socket: %w", nw.VNL, actualSocket, err)
				}
			}
		}
		args = append(args, "-netdev", fmt.Sprintf("vde,id=net%d,sock=%s", i+1, vdeSock))
		args = append(args, "-device", fmt.Sprintf("virtio-net-pci,netdev=net%d,mac=%s", i+1, nw.MACAddress))
	}

	// virtio-rng-pci accelerates starting up the OS, according to https://wiki.gentoo.org/wiki/QEMU/Options
	args = append(args, "-device", "virtio-rng-pci")

	// Graphics
	if y.Video.Display != "" {
		args = appendArgsIfNoConflict(args, "-display", y.Video.Display)
	}
	switch y.Arch {
	case limayaml.X8664:
		args = append(args, "-device", "virtio-vga")
		args = append(args, "-device", "virtio-keyboard-pci")
		args = append(args, "-device", "virtio-mouse-pci")
	default:
		// QEMU does not seem to support virtio-vga for aarch64
		args = append(args, "-vga", "none", "-device", "ramfb")
		args = append(args, "-device", "usb-ehci")
		args = append(args, "-device", "usb-kbd")
		args = append(args, "-device", "usb-mouse")
	}

	// Parallel
	args = append(args, "-parallel", "none")

	// Serial
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

	// We also want to enable vsock and virtfs here, but QEMU does not support vsock and virtfs for macOS hosts

	// QMP
	qmpSock := filepath.Join(cfg.InstanceDir, filenames.QMPSock)
	if err := os.RemoveAll(qmpSock); err != nil {
		return "", nil, err
	}
	const qmpChardev = "char-qmp"
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server=on,wait=off", qmpChardev, qmpSock))
	args = append(args, "-qmp", "chardev:"+qmpChardev)

	// QEMU process
	args = append(args, "-name", "lima-"+cfg.Name)
	args = append(args, "-pidfile", filepath.Join(cfg.InstanceDir, filenames.QemuPID))

	return exe, args, nil
}

func getExe(arch limayaml.Arch) (string, []string, error) {
	exeBase := "qemu-system-" + arch
	var args []string
	envK := "QEMU_SYSTEM_" + strings.ToUpper(arch)
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
	exe, err := exec.LookPath(exeBase)
	if err != nil {
		return "", nil, err
	}
	return exe, args, nil
}

func isNativeArch(arch limayaml.Arch) bool {
	nativeX8664 := arch == limayaml.X8664 && runtime.GOARCH == "amd64"
	nativeAARCH64 := arch == limayaml.AARCH64 && runtime.GOARCH == "arm64"
	return nativeX8664 || nativeAARCH64
}

func getAccel(arch limayaml.Arch) string {
	if isNativeArch(arch) {
		switch runtime.GOOS {
		case "darwin":
			return "hvf"
		case "linux":
			return "kvm"
		case "netbsd":
			return "nvmm" // untested
		case "windows":
			return "whpx" // untested
		}
	}
	return "tcg"
}

func getFirmware(qemuExe string, arch limayaml.Arch) (string, error) {
	binDir := filepath.Dir(qemuExe)  // "/usr/local/bin"
	localDir := filepath.Dir(binDir) // "/usr/local"

	candidates := []string{
		filepath.Join(localDir, fmt.Sprintf("share/qemu/edk2-%s-code.fd", arch)), // macOS (homebrew)
	}

	switch arch {
	case limayaml.X8664:
		// Debian package "ovmf"
		candidates = append(candidates, "/usr/share/OVMF/OVMF_CODE.fd")
		// openSUSE package "qemu-ovmf-x86_64"
		candidates = append(candidates, "/usr/share/qemu/ovmf-x86_64-code.bin")
	case limayaml.AARCH64:
		// Debian package "qemu-efi-aarch64"
		candidates = append(candidates, "/usr/share/AAVMF/AAVMF_CODE.fd")
		// Debian package "qemu-efi-aarch64" (unpadded, backwards compatibility)
		candidates = append(candidates, "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd")
	}

	logrus.Debugf("firmware candidates = %v", candidates)

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}

	if arch == limayaml.X8664 {
		return "", fmt.Errorf("could not find firmware for %q (hint: try setting `firmware.legacyBIOS` to `true`)", qemuExe)
	}
	return "", fmt.Errorf("could not find firmware for %q", qemuExe)
}
