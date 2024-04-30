package limayaml

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/docker/go-units"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/sirupsen/logrus"
)

func validateFileObject(f File, fieldName string) error {
	if !strings.Contains(f.Location, "://") {
		if _, err := localpathutil.Expand(f.Location); err != nil {
			return fmt.Errorf("field `%s.location` refers to an invalid local file path: %q: %w", fieldName, f.Location, err)
		}
		// f.Location does NOT need to be accessible, so we do NOT check os.Stat(f.Location)
	}
	switch f.Arch {
	case X8664, AARCH64, ARMV7L, RISCV64:
	default:
		return fmt.Errorf("field `arch` must be %q, %q, %q, or %q; got %q", X8664, AARCH64, ARMV7L, RISCV64, f.Arch)
	}
	if f.Digest != "" {
		if !f.Digest.Algorithm().Available() {
			return fmt.Errorf("field `%s.digest` refers to an unavailable digest algorithm", fieldName)
		}
		if err := f.Digest.Validate(); err != nil {
			return fmt.Errorf("field `%s.digest` is invalid: %s: %w", fieldName, f.Digest.String(), err)
		}
	}
	return nil
}

func Validate(y LimaYAML, warn bool) error {
	switch *y.OS {
	case LINUX:
	default:
		return fmt.Errorf("field `os` must be %q; got %q", LINUX, *y.OS)
	}
	switch *y.Arch {
	case X8664, AARCH64, ARMV7L, RISCV64:
	default:
		return fmt.Errorf("field `arch` must be %q, %q, %q or %q; got %q", X8664, AARCH64, ARMV7L, RISCV64, *y.Arch)
	}

	switch *y.VMType {
	case QEMU:
		// NOP
	case WSL2:
		// NOP
	case VZ:
		if !IsNativeArch(*y.Arch) {
			return fmt.Errorf("field `arch` must be %q for VZ; got %q", NewArch(runtime.GOARCH), *y.Arch)
		}
	default:
		return fmt.Errorf("field `vmType` must be %q, %q, %q; got %q", QEMU, VZ, WSL2, *y.VMType)
	}

	if len(y.Images) == 0 {
		return errors.New("field `images` must be set")
	}
	for i, f := range y.Images {
		if err := validateFileObject(f.File, fmt.Sprintf("images[%d]", i)); err != nil {
			return err
		}
		if f.Kernel != nil {
			if err := validateFileObject(f.Kernel.File, fmt.Sprintf("images[%d].kernel", i)); err != nil {
				return err
			}
			if f.Kernel.Arch != f.Arch {
				return fmt.Errorf("images[%d].kernel has unexpected architecture %q, must be %q", i, f.Kernel.Arch, f.Arch)
			}
		} else if f.Arch == RISCV64 {
			return errors.New("riscv64 needs the kernel (e.g., \"uboot.elf\") to be specified")
		}
		if f.Initrd != nil {
			if err := validateFileObject(*f.Initrd, fmt.Sprintf("images[%d].initrd", i)); err != nil {
				return err
			}
			if f.Kernel == nil {
				return errors.New("initrd requires the kernel to be specified")
			}
			if f.Initrd.Arch != f.Arch {
				return fmt.Errorf("images[%d].initrd has unexpected architecture %q, must be %q", i, f.Initrd.Arch, f.Arch)
			}
		}
	}

	for arch := range y.CPUType {
		switch arch {
		case AARCH64, X8664, ARMV7L, RISCV64:
			// these are the only supported architectures
		default:
			return fmt.Errorf("field `cpuType` uses unsupported arch %q", arch)
		}
	}

	if *y.CPUs == 0 {
		return errors.New("field `cpus` must be set")
	}

	if (*y.CPUs > runtime.NumCPU()) {
		return fmt.Errorf("field `cpus` is set to %d, which is greater than the number of CPUs available (%d)", *y.CPUs, runtime.NumCPU())
	}

	if _, err := units.RAMInBytes(*y.Memory); err != nil {
		return fmt.Errorf("field `memory` has an invalid value: %w", err)
	}

	if _, err := units.RAMInBytes(*y.Disk); err != nil {
		return fmt.Errorf("field `memory` has an invalid value: %w", err)
	}

	u, err := osutil.LimaUser(false)
	if err != nil {
		return fmt.Errorf("internal error (not an error of YAML): %w", err)
	}
	// reservedHome is the home directory defined in "cidata.iso:/user-data"
	reservedHome := fmt.Sprintf("/home/%s.linux", u.Username)

	for i, f := range y.Mounts {
		if !filepath.IsAbs(f.Location) && !strings.HasPrefix(f.Location, "~") {
			return fmt.Errorf("field `mounts[%d].location` must be an absolute path, got %q",
				i, f.Location)
		}
		loc, err := localpathutil.Expand(f.Location)
		if err != nil {
			return fmt.Errorf("field `mounts[%d].location` refers to an unexpandable path: %q: %w", i, f.Location, err)
		}
		switch loc {
		case "/", "/bin", "/dev", "/etc", "/home", "/opt", "/sbin", "/tmp", "/usr", "/var":
			return fmt.Errorf("field `mounts[%d].location` must not be a system path such as /etc or /usr", i)
		case reservedHome:
			return fmt.Errorf("field `mounts[%d].location` is internally reserved", i)
		}

		st, err := os.Stat(loc)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("field `mounts[%d].location` refers to an inaccessible path: %q: %w", i, f.Location, err)
			}
		} else if !st.IsDir() {
			return fmt.Errorf("field `mounts[%d].location` refers to a non-directory path: %q: %w", i, f.Location, err)
		}

		if _, err := units.RAMInBytes(*f.NineP.Msize); err != nil {
			return fmt.Errorf("field `msize` has an invalid value: %w", err)
		}
	}

	if *y.SSH.LocalPort != 0 {
		if err := validatePort("ssh.localPort", *y.SSH.LocalPort); err != nil {
			return err
		}
	}

	switch *y.MountType {
	case REVSSHFS, NINEP, VIRTIOFS, WSLMount:
	default:
		return fmt.Errorf("field `mountType` must be %q or %q or %q, or %q, got %q", REVSSHFS, NINEP, VIRTIOFS, WSLMount, *y.MountType)
	}

	if warn && runtime.GOOS != "linux" {
		for i, mount := range y.Mounts {
			if mount.Virtiofs.QueueSize != nil {
				logrus.Warnf("field mounts[%d].virtiofs.queueSize is only supported on Linux", i)
			}
		}
	}

	// y.Firmware.LegacyBIOS is ignored for aarch64, but not a fatal error.

	for i, p := range y.Provision {
		switch p.Mode {
		case ProvisionModeSystem, ProvisionModeUser, ProvisionModeBoot:
			if p.SkipDefaultDependencyResolution != nil {
				return fmt.Errorf("field `provision[%d].mode` cannot set skipDefaultDependencyResolution, only valid on scripts of type %q",
					i, ProvisionModeDependency)
			}
		case ProvisionModeDependency:
		default:
			return fmt.Errorf("field `provision[%d].mode` must one of %q, %q, %q, or %q",
				i, ProvisionModeSystem, ProvisionModeUser, ProvisionModeBoot, ProvisionModeDependency)
		}
		if strings.Contains(p.Script, "LIMA_CIDATA") {
			logrus.Warn("provisioning scripts should not reference the LIMA_CIDATA variables")
		}
	}
	needsContainerdArchives := (y.Containerd.User != nil && *y.Containerd.User) || (y.Containerd.System != nil && *y.Containerd.System)
	if needsContainerdArchives && len(y.Containerd.Archives) == 0 {
		return fmt.Errorf("field `containerd.archives` must be provided")
	}
	for i, p := range y.Probes {
		switch p.Mode {
		case ProbeModeReadiness:
		default:
			return fmt.Errorf("field `probe[%d].mode` can only be %q",
				i, ProbeModeReadiness)
		}
	}
	for i, rule := range y.PortForwards {
		field := fmt.Sprintf("portForwards[%d]", i)
		if rule.GuestIPMustBeZero && !rule.GuestIP.Equal(net.IPv4zero) {
			return fmt.Errorf("field `%s.guestIPMustBeZero` can only be true when field `%s.guestIP` is 0.0.0.0", field, field)
		}
		if rule.GuestPort != 0 {
			if rule.GuestSocket != "" {
				return fmt.Errorf("field `%s.guestPort` must be 0 when field `%s.guestSocket` is set", field, field)
			}
			if rule.GuestPort != rule.GuestPortRange[0] {
				return fmt.Errorf("field `%s.guestPort` must match field `%s.guestPortRange[0]`", field, field)
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".guestPort", rule.GuestPort); err != nil {
				return err
			}
		}
		if rule.HostPort != 0 {
			if rule.HostSocket != "" {
				return fmt.Errorf("field `%s.hostPort` must be 0 when field `%s.hostSocket` is set", field, field)
			}
			if rule.HostPort != rule.HostPortRange[0] {
				return fmt.Errorf("field `%s.hostPort` must match field `%s.hostPortRange[0]`", field, field)
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".hostPort", rule.HostPort); err != nil {
				return err
			}
		}
		for j := 0; j < 2; j++ {
			if err := validatePort(fmt.Sprintf("%s.guestPortRange[%d]", field, j), rule.GuestPortRange[j]); err != nil {
				return err
			}
			if err := validatePort(fmt.Sprintf("%s.hostPortRange[%d]", field, j), rule.HostPortRange[j]); err != nil {
				return err
			}
		}
		if rule.GuestPortRange[0] > rule.GuestPortRange[1] {
			return fmt.Errorf("field `%s.guestPortRange[1]` must be greater than or equal to field `%s.guestPortRange[0]`", field, field)
		}
		if rule.HostPortRange[0] > rule.HostPortRange[1] {
			return fmt.Errorf("field `%s.hostPortRange[1]` must be greater than or equal to field `%s.hostPortRange[0]`", field, field)
		}
		if rule.GuestPortRange[1]-rule.GuestPortRange[0] != rule.HostPortRange[1]-rule.HostPortRange[0] {
			return fmt.Errorf("field `%s.hostPortRange` must specify the same number of ports as field `%s.guestPortRange`", field, field)
		}
		if rule.GuestSocket != "" {
			if !path.IsAbs(rule.GuestSocket) {
				return fmt.Errorf("field `%s.guestSocket` must be an absolute path", field)
			}
			if rule.HostSocket == "" && rule.HostPortRange[1]-rule.HostPortRange[0] > 0 {
				return fmt.Errorf("field `%s.guestSocket` can only be mapped to a single port or socket. not a range", field)
			}
		}
		if rule.HostSocket != "" {
			if !filepath.IsAbs(rule.HostSocket) {
				// should be unreachable because FillDefault() will prepend the instance directory to relative names
				return fmt.Errorf("field `%s.hostSocket` must be an absolute path, but is %q", field, rule.HostSocket)
			}
			if rule.GuestSocket == "" && rule.GuestPortRange[1]-rule.GuestPortRange[0] > 0 {
				return fmt.Errorf("field `%s.hostSocket` can only be mapped from a single port or socket. not a range", field)
			}
		}
		if len(rule.HostSocket) >= osutil.UnixPathMax {
			return fmt.Errorf("field `%s.hostSocket` must be less than UNIX_PATH_MAX=%d characters, but is %d",
				field, osutil.UnixPathMax, len(rule.HostSocket))
		}
		if rule.Proto != TCP {
			return fmt.Errorf("field `%s.proto` must be %q", field, TCP)
		}
		if rule.Reverse && rule.GuestSocket == "" {
			return fmt.Errorf("field `%s.reverse` must be %t", field, false)
		}
		if rule.Reverse && rule.HostSocket == "" {
			return fmt.Errorf("field `%s.reverse` must be %t", field, false)
		}
		// Not validating that the various GuestPortRanges and HostPortRanges are not overlapping. Rules will be
		// processed sequentially and the first matching rule for a guest port determines forwarding behavior.
	}
	for i, rule := range y.CopyToHost {
		field := fmt.Sprintf("CopyToHost[%d]", i)
		if rule.GuestFile != "" {
			if !path.IsAbs(rule.GuestFile) {
				return fmt.Errorf("field `%s.guest` must be an absolute path", field)
			}
		}
		if rule.HostFile != "" {
			if !filepath.IsAbs(rule.HostFile) {
				return fmt.Errorf("field `%s.host` must be an absolute path, but is %q", field, rule.HostFile)
			}
		}
	}

	if y.HostResolver.Enabled != nil && *y.HostResolver.Enabled && len(y.DNS) > 0 {
		return fmt.Errorf("field `dns` must be empty when field `HostResolver.Enabled` is true")
	}

	if err := validateNetwork(y, warn); err != nil {
		return err
	}
	if warn {
		warnExperimental(y)
	}
	return nil
}

func validateNetwork(y LimaYAML, warn bool) error {
	interfaceName := make(map[string]int)
	for i, nw := range y.Networks {
		field := fmt.Sprintf("networks[%d]", i)
		if nw.Lima != "" {
			config, err := networks.Config()
			if err != nil {
				return err
			}
			if config.Check(nw.Lima) != nil {
				return fmt.Errorf("field `%s.lima` references network %q which is not defined in networks.yaml", field, nw.Lima)
			}
			usernet, err := config.Usernet(nw.Lima)
			if err != nil {
				return err
			}
			if !usernet && runtime.GOOS != "darwin" {
				return fmt.Errorf("field `%s.lima` is only supported on macOS right now", field)
			}
			if nw.Socket != "" {
				return fmt.Errorf("field `%s.lima` and field `%s.socket` are mutually exclusive", field, field)
			}
			if nw.VZNAT != nil && *nw.VZNAT {
				return fmt.Errorf("field `%s.lima` and field `%s.vzNAT` are mutually exclusive", field, field)
			}
			if nw.VNLDeprecated != "" {
				return fmt.Errorf("field `%s.lima` and field `%s.vnl` are mutually exclusive", field, field)
			}
			if nw.SwitchPortDeprecated != 0 {
				return fmt.Errorf("field `%s.switchPort` cannot be used with field `%s.lima`", field, field)
			}
		} else if nw.Socket != "" {
			if nw.VZNAT != nil && *nw.VZNAT {
				return fmt.Errorf("field `%s.socket` and field `%s.vzNAT` are mutually exclusive", field, field)
			}
			if nw.VNLDeprecated != "" {
				return fmt.Errorf("field `%s.socket` and field `%s.vnl` are mutually exclusive", field, field)
			}
			if nw.SwitchPortDeprecated != 0 {
				return fmt.Errorf("field `%s.switchPort` cannot be used with field `%s.socket`", field, field)
			}
			if fi, err := os.Stat(nw.Socket); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			} else if err == nil && fi.Mode()&os.ModeSocket == 0 {
				return fmt.Errorf("field `%s.socket` %q points to a non-socket file", field, nw.Socket)
			}
		} else if nw.VZNAT != nil && *nw.VZNAT {
			if y.VMType == nil || *y.VMType != VZ {
				return fmt.Errorf("field `%s.vzNAT` requires `vmType` to be %q", field, VZ)
			}
			if nw.Lima != "" {
				return fmt.Errorf("field `%s.vzNAT` and field `%s.lima` are mutually exclusive", field, field)
			}
			if nw.Socket != "" {
				return fmt.Errorf("field `%s.vzNAT` and field `%s.socket` are mutually exclusive", field, field)
			}
			if nw.VNLDeprecated != "" {
				return fmt.Errorf("field `%s.vzNAT` and field `%s.vnl` are mutually exclusive", field, field)
			}
			if nw.SwitchPortDeprecated != 0 {
				return fmt.Errorf("field `%s.switchPort` cannot be used with field `%s.vzNAT`", field, field)
			}
		} else {
			if nw.VNLDeprecated == "" {
				return fmt.Errorf("field `%s.lima`, field `%s.socket`, or field `%s.vnl` must be set", field, field, field)
			}
			// The field is called VDE.VNL in anticipation of QEMU upgrading VDE2 to VDEplug4,
			// but right now the only valid value on macOS is a path to the vde_switch socket directory,
			// optionally with vde:// prefix.
			if !strings.Contains(nw.VNLDeprecated, "://") || strings.HasPrefix(nw.VNLDeprecated, "vde://") {
				vdeSwitch := strings.TrimPrefix(nw.VNLDeprecated, "vde://")
				if fi, err := os.Stat(vdeSwitch); err != nil {
					// negligible when the instance is stopped
					logrus.WithError(err).Debugf("field `%s.vnl` %q failed stat", field, vdeSwitch)
				} else {
					if fi.IsDir() {
						/* Switch mode (vdeSwitch is dir, port != 65535) */
						ctlSocket := filepath.Join(vdeSwitch, "ctl")
						// ErrNotExist during os.Stat(ctlSocket) can be ignored. ctlSocket does not need to exist until actually starting the VM
						if fi, err = os.Stat(ctlSocket); err == nil {
							if fi.Mode()&os.ModeSocket == 0 {
								return fmt.Errorf("field `%s.vnl` file %q is not a UNIX socket", field, ctlSocket)
							}
						}
						if nw.SwitchPortDeprecated == 65535 {
							return fmt.Errorf("field `%s.vnl` points to a non-PTP switch, so the port number must not be 65535", field)
						}
					} else {
						/* PTP mode (vdeSwitch is socket, port == 65535) */
						if fi.Mode()&os.ModeSocket == 0 {
							return fmt.Errorf("field `%s.vnl` %q is not a directory nor a UNIX socket", field, vdeSwitch)
						}
						if nw.SwitchPortDeprecated != 65535 {
							return fmt.Errorf("field `%s.vnl` points to a PTP (switchless) socket %q, so the port number has to be 65535 (got %d)",
								field, vdeSwitch, nw.SwitchPortDeprecated)
						}
					}
				}
			} else if runtime.GOOS != "linux" {
				if warn {
					logrus.Warnf("field `%s.vnl` is unlikely to work for %s (unless libvdeplug4 has been ported to %s and is installed)",
						field, runtime.GOOS, runtime.GOOS)
				}
			}
		}
		if nw.MACAddress != "" {
			hw, err := net.ParseMAC(nw.MACAddress)
			if err != nil {
				return fmt.Errorf("field `vmnet.mac` invalid: %w", err)
			}
			if len(hw) != 6 {
				return fmt.Errorf("field `%s.macAddress` must be a 48 bit (6 bytes) MAC address; actual length of %q is %d bytes", field, nw.MACAddress, len(hw))
			}
		}
		// FillDefault() will make sure that nw.Interface is not the empty string
		if len(nw.Interface) >= 16 {
			return fmt.Errorf("field `%s.interface` must be less than 16 bytes, but is %d bytes: %q", field, len(nw.Interface), nw.Interface)
		}
		if strings.ContainsAny(nw.Interface, " \t\n/") {
			return fmt.Errorf("field `%s.interface` must not contain whitespace or slashes", field)
		}
		if nw.Interface == networks.SlirpNICName {
			return fmt.Errorf("field `%s.interface` must not be set to %q because it is reserved for slirp", field, networks.SlirpNICName)
		}
		if prev, ok := interfaceName[nw.Interface]; ok {
			return fmt.Errorf("field `%s.interface` value %q has already been used by field `networks[%d].interface`", field, nw.Interface, prev)
		}
		interfaceName[nw.Interface] = i
	}
	return nil
}

func validatePort(field string, port int) error {
	switch {
	case port < 0:
		return fmt.Errorf("field `%s` must be > 0", field)
	case port == 0:
		return fmt.Errorf("field `%s` must be set", field)
	case port == 22:
		return fmt.Errorf("field `%s` must not be 22", field)
	case port > 65535:
		return fmt.Errorf("field `%s` must be < 65536", field)
	}
	return nil
}

func warnExperimental(y LimaYAML) {
	if *y.MountType == NINEP {
		logrus.Warn("`mountType: 9p` is experimental")
	}
	if *y.MountType == VIRTIOFS && runtime.GOOS == "linux" {
		logrus.Warn("`mountType: virtiofs` on Linux is experimental")
	}
	if *y.VMType == VZ {
		logrus.Warn("`vmType: vz` is experimental")
	}
	if *y.Arch == RISCV64 {
		logrus.Warn("`arch: riscv64` is experimental")
	}
	if y.Video.Display != nil && strings.Contains(*y.Video.Display, "vnc") {
		logrus.Warn("`video.display: vnc` is experimental")
	}
	if y.Audio.Device != nil && *y.Audio.Device != "" {
		logrus.Warn("`audio.device` is experimental")
	}
	if y.MountInotify != nil && *y.MountInotify {
		logrus.Warn("`mountInotify` is experimental")
	}
}
