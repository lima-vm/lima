package qemu

import (
	"crypto/sha256"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/downloader"
	"github.com/AkihiroSuda/lima/pkg/iso9660util"
	"github.com/AkihiroSuda/lima/pkg/limayaml"
	"github.com/AkihiroSuda/lima/pkg/store/filenames"
	"github.com/docker/go-units"
	"github.com/mattn/go-shellwords"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Name        string
	InstanceDir string
	LimaYAML    *limayaml.LimaYAML
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
			logrus.Infof("Attempting to download the image from %q", f.Location)
			res, err := downloader.Download(baseDisk, f.Location,
				downloader.WithCache(),
				downloader.WithExpectedDigest(f.Digest),
			)
			if err != nil {
				errs[i] = errors.Wrapf(err, "failed to download %q", f.Location)
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
			return errors.Errorf("failed to download the image, attempted %d candidates, errors=%v",
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
		args = append(args, "-b", baseDisk)
	}
	args = append(args, diffDisk, strconv.Itoa(int(diskSize)))
	cmd := exec.Command("qemu-img", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "failed to run %v: %q", cmd.Args, string(out))
	}
	return nil
}

func argValue(args []string, key string) (string, bool) {
	if !strings.HasPrefix(key, "-") {
		panic(errors.Errorf("got unexpected key %q", key))
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
		panic(errors.Errorf("got unexpected key %q", k))
	}
	switch k {
	case "-drive", "-cdrom", "-chardev", "-blockdev", "-netdev", "-device":
		panic(errors.Errorf("appendArgsIfNoConflict() must not be called with k=%q", k))
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

func macAddress(cfg Config) string {
	addr := cfg.LimaYAML.Network.VDE.MACAddress
	if addr == "" {
		sha := sha256.Sum256([]byte(cfg.InstanceDir))
		// According to https://gitlab.com/wireshark/wireshark/-/blob/master/manuf
		// no well-known MAC addresses start with 0x22.
		hw := append(net.HardwareAddr{0x22}, sha[0:5]...)
		addr = hw.String()
	}
	return addr
}

func Cmdline(cfg Config) (string, []string, error) {
	y := cfg.LimaYAML
	exe, args, err := getExe(y.Arch)
	if err != nil {
		return "", nil, err
	}

	// Architecture
	accel := getAccel(y.Arch)
	switch y.Arch {
	case limayaml.X8664:
		// NOTE: "-cpu host" seems to cause kernel panic
		// (MacBookPro 2020, Intel(R) Core(TM) i7-1068NG7 CPU @ 2.30GHz, macOS 11.3, Ubuntu 21.04)
		args = appendArgsIfNoConflict(args, "-cpu", "Haswell-v4")
		args = appendArgsIfNoConflict(args, "-machine", "q35,accel="+accel)
	case limayaml.AARCH64:
		args = appendArgsIfNoConflict(args, "-cpu", "cortex-a72")
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
	if !y.Firmware.LegacyBIOS {
		firmware, err := getFirmware(exe, y.Arch)
		if err != nil {
			return "", nil, err
		}
		args = append(args, "-drive", fmt.Sprintf("if=pflash,format=raw,readonly,file=%s", firmware))
	} else if y.Arch != limayaml.X8664 {
		logrus.Warnf("field `firmware.legacyBIOS` is not supported for architecture %q, ignoring", y.Arch)
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
	if y.Network.VDE.URL != "" {
		args = append(args, "-device", fmt.Sprintf("virtio-net-pci,netdev=net0,mac=%s", macAddress(cfg)))
		args = append(args, "-netdev", fmt.Sprintf("vde,id=net0,sock=%s", y.Network.VDE.URL))
	}
	// CIDR is intentionally hardcoded to 192.168.5.0/24, as each of QEMU has its own independent slirp network.
	args = append(args, "-device", "virtio-net-pci,netdev=net1")
	args = append(args, "-netdev", fmt.Sprintf("user,id=net1,net=192.168.5.0/24,hostfwd=tcp:127.0.0.1:%d-:22", y.SSH.LocalPort))

	// virtio-rng-pci acceralates starting up the OS, according to https://wiki.gentoo.org/wiki/QEMU/Options
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
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server,nowait,logfile=%s", serialChardev, serialSock, serialLog))
	args = append(args, "-serial", "chardev:"+serialChardev)

	// We also want to enable vsock and virtfs here, but QEMU does not support vsock and virtfs for macOS hosts

	// QMP
	qmpSock := filepath.Join(cfg.InstanceDir, filenames.QMPSock)
	if err := os.RemoveAll(qmpSock); err != nil {
		return "", nil, err
	}
	const qmpChardev = "char-qmp"
	args = append(args, "-chardev", fmt.Sprintf("socket,id=%s,path=%s,server,nowait", qmpChardev, qmpSock))
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
			return "", nil, errors.Wrapf(err, "failed to parse %s value %q", envK, envV)
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

func getAccel(arch limayaml.Arch) string {
	nativeX8664 := arch == limayaml.X8664 && runtime.GOARCH == "amd64"
	nativeAARCH64 := arch == limayaml.AARCH64 && runtime.GOARCH == "arm64"
	native := nativeX8664 || nativeAARCH64
	if native {
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
	case limayaml.AARCH64:
		// Debian package "qemu-efi-aarch64"
		candidates = append(candidates, "/usr/share/qemu-efi-aarch64/QEMU_EFI.fd")
	}

	logrus.Debugf("firmware candidates = %v", candidates)

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			return f, nil
		}
	}

	if arch == limayaml.X8664 {
		return "", errors.Errorf("could not find firmware for %q (hint: try setting `firmware.legacyBIOS` to `true`)", qemuExe)
	}
	return "", errors.Errorf("could not find firmware for %q", qemuExe)
}
