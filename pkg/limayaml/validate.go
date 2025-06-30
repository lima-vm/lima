// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/coreos/go-semver/semver"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/pkg/identifiers"
	"github.com/lima-vm/lima/pkg/localpathutil"
	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/version"
	"github.com/lima-vm/lima/pkg/version/versionutil"
)

func Validate(y *LimaYAML, warn bool) error {
	var errs error

	if len(y.Base) > 0 {
		errs = errors.Join(errs, errors.New("field `base` must be empty for YAML validation"))
	}

	if y.MinimumLimaVersion != nil {
		if _, err := versionutil.Parse(*y.MinimumLimaVersion); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `minimumLimaVersion` must be a semvar value, got %q: %w", *y.MinimumLimaVersion, err))
		} else {
			limaVersion, err := versionutil.Parse(version.Version)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("can't parse builtin Lima version %q: %w", version.Version, err))
			} else if versionutil.GreaterThan(*y.MinimumLimaVersion, limaVersion.String()) {
				errs = errors.Join(errs, fmt.Errorf("template requires Lima version %q; this is only %q", *y.MinimumLimaVersion, limaVersion.String()))
			}
		}
	}
	if y.VMOpts.QEMU.MinimumVersion != nil {
		if _, err := semver.NewVersion(*y.VMOpts.QEMU.MinimumVersion); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `vmOpts.qemu.minimumVersion` must be a semvar value, got %q: %w", *y.VMOpts.QEMU.MinimumVersion, err))
		}
	}
	switch *y.OS {
	case LINUX:
	default:
		errs = errors.Join(errs, fmt.Errorf("field `os` must be %q; got %q", LINUX, *y.OS))
	}
	if !slices.Contains(ArchTypes, *y.Arch) {
		errs = errors.Join(errs, fmt.Errorf("field `arch` must be one of %v; got %q", ArchTypes, *y.Arch))
	}
	switch *y.VMType {
	case QEMU:
		// NOP
	case WSL2:
		// NOP
	case VZ:
		if !IsNativeArch(*y.Arch) {
			errs = errors.Join(errs, fmt.Errorf("field `arch` must be %q for VZ; got %q", NewArch(runtime.GOARCH), *y.Arch))
		}
	default:
		errs = errors.Join(errs, fmt.Errorf("field `vmType` must be %q, %q, %q; got %q", QEMU, VZ, WSL2, *y.VMType))
	}

	if len(y.Images) == 0 {
		errs = errors.Join(errs, errors.New("field `images` must be set"))
	}
	for i, f := range y.Images {
		err := validateFileObject(f.File, fmt.Sprintf("images[%d]", i))
		if err != nil {
			errs = errors.Join(errs, err)
		}
		if f.Kernel != nil {
			err := validateFileObject(f.Kernel.File, fmt.Sprintf("images[%d].kernel", i))
			if err != nil {
				errs = errors.Join(errs, err)
			}
			if f.Kernel.Arch != f.Arch {
				errs = errors.Join(errs, fmt.Errorf("images[%d].kernel has unexpected architecture %q, must be %q", i, f.Kernel.Arch, f.Arch))
			}
		}
		if f.Initrd != nil {
			err := validateFileObject(*f.Initrd, fmt.Sprintf("images[%d].initrd", i))
			if err != nil {
				errs = errors.Join(errs, err)
			}
			if f.Initrd.Arch != f.Arch {
				errs = errors.Join(errs, fmt.Errorf("images[%d].initrd has unexpected architecture %q, must be %q", i, f.Initrd.Arch, f.Arch))
			}
		}
	}

	for arch := range y.CPUType {
		if !slices.Contains(ArchTypes, arch) {
			errs = errors.Join(errs, fmt.Errorf("field `cpuType` uses unsupported arch %q", arch))
		}
	}

	if *y.CPUs == 0 {
		errs = errors.Join(errs, errors.New("field `cpus` must be set"))
	}

	if _, err := units.RAMInBytes(*y.Memory); err != nil {
		errs = errors.Join(errs, fmt.Errorf("field `memory` has an invalid value: %w", err))
	}

	if _, err := units.RAMInBytes(*y.Disk); err != nil {
		errs = errors.Join(errs, fmt.Errorf("field `disk` has an invalid value: %w", err))
	}

	for i, disk := range y.AdditionalDisks {
		if err := identifiers.Validate(disk.Name); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `additionalDisks[%d].name is invalid`: %w", i, err))
		}
	}

	for i, f := range y.Mounts {
		if !filepath.IsAbs(f.Location) && !strings.HasPrefix(f.Location, "~") {
			errs = errors.Join(errs, fmt.Errorf("field `mounts[%d].location` must be an absolute path, got %q",
				i, f.Location))
		}
		// f.Location has already been expanded in FillDefaults(), but that function cannot return errors.
		loc, err := localpathutil.Expand(f.Location)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `mounts[%d].location` refers to an unexpandable path: %q: %w", i, f.Location, err))
		}
		st, err := os.Stat(loc)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				errs = errors.Join(errs, fmt.Errorf("field `mounts[%d].location` refers to an inaccessible path: %q: %w", i, f.Location, err))
			}
			if warn {
				logrus.Warnf("field `mounts[%d].location` refers to a non-existent directory: %q:", i, f.Location)
			}
		} else if !st.IsDir() {
			errs = errors.Join(errs, fmt.Errorf("field `mounts[%d].location` refers to a non-directory path: %q: %w", i, f.Location, err))
		}

		switch *f.MountPoint {
		case "/", "/bin", "/dev", "/etc", "/home", "/opt", "/sbin", "/tmp", "/usr", "/var":
			errs = errors.Join(errs, fmt.Errorf("field `mounts[%d].mountPoint` must not be a system path such as /etc or /usr", i))
		// home directory defined in "cidata.iso:/user-data"
		case *y.User.Home:
			errs = errors.Join(errs, fmt.Errorf("field `mounts[%d].mountPoint` is the reserved internal home directory %q", i, *y.User.Home))
		}
		// There is no tilde-expansion for guest filenames
		if strings.HasPrefix(*f.MountPoint, "~") {
			errs = errors.Join(errs, fmt.Errorf("field `mounts[%d].mountPoint` must not start with \"~\"", i))
		}

		if _, err := units.RAMInBytes(*f.NineP.Msize); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `msize` has an invalid value: %w", err))
		}
	}

	if *y.SSH.LocalPort != 0 {
		if err := validatePort("ssh.localPort", *y.SSH.LocalPort); err != nil {
			errs = errors.Join(errs, err)
		}
	}

	switch *y.MountType {
	case REVSSHFS, NINEP, VIRTIOFS, WSLMount:
	default:
		errs = errors.Join(errs, fmt.Errorf("field `mountType` must be %q or %q or %q, or %q, got %q", REVSSHFS, NINEP, VIRTIOFS, WSLMount, *y.MountType))
	}

	if slices.Contains(y.MountTypesUnsupported, *y.MountType) {
		errs = errors.Join(errs, fmt.Errorf("field `mountType` must not be one of %v (`mountTypesUnsupported`), got %q", y.MountTypesUnsupported, *y.MountType))
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
		if p.File != nil {
			if p.File.URL != "" {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].file.url` must be empty during validation (script should already be embedded)", i))
			}
			if p.File.Digest != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].file.digest` support is not yet implemented", i))
			}
		}
		switch p.Mode {
		case ProvisionModeSystem, ProvisionModeUser, ProvisionModeBoot, ProvisionModeData, ProvisionModeDependency, ProvisionModeAnsible:
		default:
			errs = errors.Join(errs, fmt.Errorf("field `provision[%d].mode` must one of %q, %q, %q, %q, %q, or %q",
				i, ProvisionModeSystem, ProvisionModeUser, ProvisionModeBoot, ProvisionModeData, ProvisionModeDependency, ProvisionModeAnsible))
		}
		if p.Mode != ProvisionModeDependency && p.SkipDefaultDependencyResolution != nil {
			errs = errors.Join(errs, fmt.Errorf("field `provision[%d].mode` cannot set skipDefaultDependencyResolution, only valid on scripts of type %q",
				i, ProvisionModeDependency))
		}

		// This can lead to fatal Panic if p.Path is nil, better to return an error here
		if p.Mode == ProvisionModeData {
			if p.Path == nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].path` must not be empty when mode is %q", i, ProvisionModeData))
				return errs
			}
			if !path.IsAbs(*p.Path) {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].path` must be an absolute path", i))
			}
			if p.Content == nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].content` must not be empty when mode is %q", i, ProvisionModeData))
			}
			// FillDefaults makes sure that p.Permissions is not nil
			if _, err := strconv.ParseInt(*p.Permissions, 8, 64); err != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].permissions` must be an octal number: %w", i, err))
			}
		} else {
			if p.Script == "" && p.Mode != ProvisionModeAnsible {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].script` must not be empty", i))
			}
			if p.Content != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].content` can only be set when mode is %q", i, ProvisionModeData))
			}
			if p.Overwrite != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].overwrite` can only be set when mode is %q", i, ProvisionModeData))
			}
			if p.Owner != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].owner` can only be set when mode is %q", i, ProvisionModeData))
			}
			if p.Path != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].path` can only be set when mode is %q", i, ProvisionModeData))
			}
			if p.Permissions != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].permissions` can only be set when mode is %q", i, ProvisionModeData))
			}
		}
		if p.Playbook != "" {
			if p.Mode != ProvisionModeAnsible {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].playbook can only be set when mode is %q", i, ProvisionModeAnsible))
			}
			if p.Script != "" {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].script must be empty if playbook is set", i))
			}
			playbook := p.Playbook
			if _, err := os.Stat(playbook); err != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].playbook` refers to an inaccessible path: %q: %w", i, playbook, err))
			}
			logrus.Warnf("provision mode %q is deprecated, use `ansible-playbook %q` instead", ProvisionModeAnsible, playbook)
		}
		if strings.Contains(p.Script, "LIMA_CIDATA") {
			logrus.Warn("provisioning scripts should not reference the LIMA_CIDATA variables")
		}
	}
	needsContainerdArchives := (y.Containerd.User != nil && *y.Containerd.User) || (y.Containerd.System != nil && *y.Containerd.System)
	if needsContainerdArchives {
		if len(y.Containerd.Archives) == 0 {
			errs = errors.Join(errs, errors.New("field `containerd.archives` must be provided"))
		}
		for i, f := range y.Containerd.Archives {
			err := validateFileObject(f, fmt.Sprintf("containerd.archives[%d]", i))
			if err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}
	for i, p := range y.Probes {
		if p.File != nil {
			if p.File.URL != "" {
				errs = errors.Join(errs, fmt.Errorf("field `probe[%d].file.url` must be empty during validation (script should already be embedded)", i))
			}
			if p.File.Digest != nil {
				errs = errors.Join(errs, fmt.Errorf("field `probe[%d].file.digest` support is not yet implemented", i))
			}
		}
		if !strings.HasPrefix(p.Script, "#!") {
			errs = errors.Join(errs, fmt.Errorf("field `probe[%d].script` must start with a '#!' line", i))
		}
		switch p.Mode {
		case ProbeModeReadiness:
		default:
			errs = errors.Join(errs, fmt.Errorf("field `probe[%d].mode` can only be %q", i, ProbeModeReadiness))
		}
	}
	for i, rule := range y.PortForwards {
		field := fmt.Sprintf("portForwards[%d]", i)
		if rule.GuestIPMustBeZero && !rule.GuestIP.Equal(net.IPv4zero) {
			errs = errors.Join(errs, fmt.Errorf("field `%s.guestIPMustBeZero` can only be true when field `%s.guestIP` is 0.0.0.0", field, field))
		}
		if rule.GuestPort != 0 {
			if rule.GuestSocket != "" {
				errs = errors.Join(errs, fmt.Errorf("field `%s.guestPort` must be 0 when field `%s.guestSocket` is set", field, field))
			}
			if rule.GuestPort != rule.GuestPortRange[0] {
				errs = errors.Join(errs, fmt.Errorf("field `%s.guestPort` must match field `%s.guestPortRange[0]`", field, field))
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".guestPort", rule.GuestPort); err != nil {
				errs = errors.Join(errs, err)
			}
		}
		if rule.HostPort != 0 {
			if rule.HostSocket != "" {
				errs = errors.Join(errs, fmt.Errorf("field `%s.hostPort` must be 0 when field `%s.hostSocket` is set", field, field))
			}
			if rule.HostPort != rule.HostPortRange[0] {
				errs = errors.Join(errs, fmt.Errorf("field `%s.hostPort` must match field `%s.hostPortRange[0]`", field, field))
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".hostPort", rule.HostPort); err != nil {
				errs = errors.Join(errs, err)
			}
		}
		for j := range 2 {
			if err := validatePort(fmt.Sprintf("%s.guestPortRange[%d]", field, j), rule.GuestPortRange[j]); err != nil {
				errs = errors.Join(errs, err)
			}
			if err := validatePort(fmt.Sprintf("%s.hostPortRange[%d]", field, j), rule.HostPortRange[j]); err != nil {
				errs = errors.Join(errs, err)
			}
		}
		if rule.GuestPortRange[0] > rule.GuestPortRange[1] {
			errs = errors.Join(errs, fmt.Errorf("field `%s.guestPortRange[1]` must be greater than or equal to field `%s.guestPortRange[0]`", field, field))
		}
		if rule.HostPortRange[0] > rule.HostPortRange[1] {
			errs = errors.Join(errs, fmt.Errorf("field `%s.hostPortRange[1]` must be greater than or equal to field `%s.hostPortRange[0]`", field, field))
		}
		if rule.GuestPortRange[1]-rule.GuestPortRange[0] != rule.HostPortRange[1]-rule.HostPortRange[0] {
			errs = errors.Join(errs, fmt.Errorf("field `%s.hostPortRange` must specify the same number of ports as field `%s.guestPortRange`", field, field))
		}
		if rule.GuestSocket != "" {
			if !path.IsAbs(rule.GuestSocket) {
				errs = errors.Join(errs, fmt.Errorf("field `%s.guestSocket` must be an absolute path, but is %q", field, rule.GuestSocket))
			}
			if rule.HostSocket == "" && rule.HostPortRange[1]-rule.HostPortRange[0] > 0 {
				errs = errors.Join(errs, fmt.Errorf("field `%s.guestSocket` can only be mapped to a single port or socket. not a range", field))
			}
		}
		if rule.HostSocket != "" {
			if !filepath.IsAbs(rule.HostSocket) {
				// should be unreachable because FillDefault() will prepend the instance directory to relative names
				errs = errors.Join(errs, fmt.Errorf("field `%s.hostSocket` must be an absolute path, but is %q", field, rule.HostSocket))
			}
			if rule.GuestSocket == "" && rule.GuestPortRange[1]-rule.GuestPortRange[0] > 0 {
				errs = errors.Join(errs, fmt.Errorf("field `%s.hostSocket` can only be mapped from a single port or socket. not a range", field))
			}
		}
		if len(rule.HostSocket) >= osutil.UnixPathMax {
			errs = errors.Join(errs, fmt.Errorf("field `%s.hostSocket` must be less than UNIX_PATH_MAX=%d characters, but is %d",
				field, osutil.UnixPathMax, len(rule.HostSocket)))
		}
		switch rule.Proto {
		case ProtoTCP, ProtoUDP, ProtoAny:
		default:
			errs = errors.Join(errs, fmt.Errorf("field `%s.proto` must be %q, %q, or %q", field, ProtoTCP, ProtoUDP, ProtoAny))
		}
		if rule.Reverse && rule.GuestSocket == "" {
			errs = errors.Join(errs, fmt.Errorf("field `%s.reverse` must be %t", field, false))
		}
		if rule.Reverse && rule.HostSocket == "" {
			errs = errors.Join(errs, fmt.Errorf("field `%s.reverse` must be %t", field, false))
		}
		// Not validating that the various GuestPortRanges and HostPortRanges are not overlapping. Rules will be
		// processed sequentially and the first matching rule for a guest port determines forwarding behavior.
	}
	for i, rule := range y.CopyToHost {
		field := fmt.Sprintf("CopyToHost[%d]", i)
		if rule.GuestFile != "" {
			if !path.IsAbs(rule.GuestFile) {
				errs = errors.Join(errs, fmt.Errorf("field `%s.guest` must be an absolute path, but is %q", field, rule.GuestFile))
			}
		}
		if rule.HostFile != "" {
			if !filepath.IsAbs(rule.HostFile) {
				errs = errors.Join(errs, fmt.Errorf("field `%s.host` must be an absolute path, but is %q", field, rule.HostFile))
			}
		}
	}

	if y.HostResolver.Enabled != nil && *y.HostResolver.Enabled && len(y.DNS) > 0 {
		errs = errors.Join(errs, errors.New("field `dns` must be empty when field `HostResolver.Enabled` is true"))
	}

	err := validateNetwork(y)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	if warn {
		warnExperimental(y)
	}

	// Validate Param settings
	// Names must start with a letter, followed by any number of letters, digits, or underscores
	validParamName := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
	for param, value := range y.Param {
		if !validParamName.MatchString(param) {
			errs = errors.Join(errs, fmt.Errorf("param %q name does not match regex %q", param, validParamName.String()))
		}
		for _, r := range value {
			if !unicode.IsPrint(r) && r != '\t' && r != ' ' {
				errs = errors.Join(errs, fmt.Errorf("param %q value contains unprintable character %q", param, r))
			}
		}
	}

	return errs
}

func validateFileObject(f File, fieldName string) error {
	var errs error
	if !strings.Contains(f.Location, "://") {
		if _, err := localpathutil.Expand(f.Location); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.location` refers to an invalid local file path: %q: %w", fieldName, f.Location, err))
		}
		// f.Location does NOT need to be accessible, so we do NOT check os.Stat(f.Location)
	}
	if !slices.Contains(ArchTypes, f.Arch) {
		errs = errors.Join(errs, fmt.Errorf("field `arch` must be one of %v; got %q", ArchTypes, f.Arch))
	}
	if f.Digest != "" {
		if !f.Digest.Algorithm().Available() {
			errs = errors.Join(errs, fmt.Errorf("field `%s.digest` refers to an unavailable digest algorithm", fieldName))
		}
		if err := f.Digest.Validate(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.digest` is invalid: %s: %w", fieldName, f.Digest.String(), err))
		}
	}
	return errs
}

func validateNetwork(y *LimaYAML) error {
	var errs error
	interfaceName := make(map[string]int)
	for i, nw := range y.Networks {
		field := fmt.Sprintf("networks[%d]", i)
		switch {
		case nw.Lima != "":
			nwCfg, err := networks.LoadConfig()
			if err != nil {
				return err
			}
			if nwCfg.Check(nw.Lima) != nil {
				errs = errors.Join(errs, fmt.Errorf("field `%s.lima` references network %q which is not defined in networks.yaml", field, nw.Lima))
			}
			usernet, err := nwCfg.Usernet(nw.Lima)
			if err != nil {
				return err
			}
			if !usernet && runtime.GOOS != "darwin" {
				errs = errors.Join(errs, fmt.Errorf("field `%s.lima` is only supported on macOS right now", field))
			}
			if nw.Socket != "" {
				errs = errors.Join(errs, fmt.Errorf("field `%s.lima` and field `%s.socket` are mutually exclusive", field, field))
			}
			if nw.VZNAT != nil && *nw.VZNAT {
				errs = errors.Join(errs, fmt.Errorf("field `%s.lima` and field `%s.vzNAT` are mutually exclusive", field, field))
			}
		case nw.Socket != "":
			if nw.VZNAT != nil && *nw.VZNAT {
				errs = errors.Join(errs, fmt.Errorf("field `%s.socket` and field `%s.vzNAT` are mutually exclusive", field, field))
			}
			if fi, err := os.Stat(nw.Socket); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = errors.Join(errs, err)
			} else if err == nil && fi.Mode()&os.ModeSocket == 0 {
				errs = errors.Join(errs, fmt.Errorf("field `%s.socket` %q points to a non-socket file", field, nw.Socket))
			}
		case nw.VZNAT != nil && *nw.VZNAT:
			if y.VMType == nil || *y.VMType != VZ {
				errs = errors.Join(errs, fmt.Errorf("field `%s.vzNAT` requires `vmType` to be %q", field, VZ))
			}
			if nw.Lima != "" {
				errs = errors.Join(errs, fmt.Errorf("field `%s.vzNAT` and field `%s.lima` are mutually exclusive", field, field))
			}
			if nw.Socket != "" {
				errs = errors.Join(errs, fmt.Errorf("field `%s.vzNAT` and field `%s.socket` are mutually exclusive", field, field))
			}
		default:
			errs = errors.Join(errs, fmt.Errorf("field `%s.lima` or  field `%s.socket must be set", field, field))
		}
		if nw.MACAddress != "" {
			hw, err := net.ParseMAC(nw.MACAddress)
			if err != nil {
				errs = errors.Join(errs, fmt.Errorf("field `vmnet.mac` invalid: %w", err))
			}
			if len(hw) != 6 {
				errs = errors.Join(errs, fmt.Errorf("field `%s.macAddress` must be a 48 bit (6 bytes) MAC address; actual length of %q is %d bytes", field, nw.MACAddress, len(hw)))
			}
		}
		// FillDefault() will make sure that nw.Interface is not the empty string
		if len(nw.Interface) >= 16 {
			errs = errors.Join(errs, fmt.Errorf("field `%s.interface` must be less than 16 bytes, but is %d bytes: %q", field, len(nw.Interface), nw.Interface))
		}
		if strings.ContainsAny(nw.Interface, " \t\n/") {
			errs = errors.Join(errs, fmt.Errorf("field `%s.interface` must not contain whitespace or slashes", field))
		}
		if nw.Interface == networks.SlirpNICName {
			errs = errors.Join(errs, fmt.Errorf("field `%s.interface` must not be set to %q because it is reserved for slirp", field, networks.SlirpNICName))
		}
		if prev, ok := interfaceName[nw.Interface]; ok {
			errs = errors.Join(errs, fmt.Errorf("field `%s.interface` value %q has already been used by field `networks[%d].interface`", field, nw.Interface, prev))
		}
		interfaceName[nw.Interface] = i
	}

	return errs
}

// validateParamIsUsed checks if the keys in the `param` field are used in any script, probe, copyToHost, or portForward.
// It should be called before the `y` parameter is passed to FillDefault() that execute template.
func validateParamIsUsed(y *LimaYAML) error {
	for key := range y.Param {
		re, err := regexp.Compile(`{{[^}]*\.Param\.` + key + `[^}]*}}|\bPARAM_` + key + `\b`)
		if err != nil {
			return fmt.Errorf("field to compile regexp for key %q: %w", key, err)
		}
		keyIsUsed := false
		for _, p := range y.Provision {
			if re.MatchString(p.Script) {
				keyIsUsed = true
				break
			}
			if p.Playbook != "" {
				playbook, err := os.ReadFile(p.Playbook)
				if err != nil {
					return err
				}
				if re.Match(playbook) {
					keyIsUsed = true
					break
				}
			}
		}
		for _, p := range y.Probes {
			if re.MatchString(p.Script) {
				keyIsUsed = true
				break
			}
		}
		for _, p := range y.CopyToHost {
			if re.MatchString(p.GuestFile) || re.MatchString(p.HostFile) {
				keyIsUsed = true
				break
			}
		}
		for _, p := range y.PortForwards {
			if re.MatchString(p.GuestSocket) || re.MatchString(p.HostSocket) {
				keyIsUsed = true
				break
			}
		}
		for _, p := range y.Mounts {
			if re.MatchString(p.Location) {
				keyIsUsed = true
				break
			}
			if p.MountPoint != nil && re.MatchString(*p.MountPoint) {
				keyIsUsed = true
				break
			}
		}
		if !keyIsUsed {
			return fmt.Errorf("field `param` key %q is not used in any provision, probe, copyToHost, or portForward", key)
		}
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

func warnExperimental(y *LimaYAML) {
	if *y.MountType == VIRTIOFS && runtime.GOOS == "linux" {
		logrus.Warn("`mountType: virtiofs` on Linux is experimental")
	}
	switch *y.Arch {
	case RISCV64, ARMV7L, S390X, PPC64LE:
		logrus.Warnf("`arch: %s ` is experimental", *y.Arch)
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

// ValidateAgainstLatestConfig validates the values between the latest YAML and the updated(New) YAML.
// This validates configuration rules that disallow certain changes, such as shrinking the disk.
func ValidateAgainstLatestConfig(yNew, yLatest []byte) error {
	var n LimaYAML

	// Load the latest YAML and fill in defaults
	l, err := LoadWithWarnings(yLatest, "")
	if err != nil {
		return err
	}
	if err := Unmarshal(yNew, &n, "Unmarshal new YAML bytes"); err != nil {
		return err
	}

	// Handle editing the template without a disk value
	if n.Disk == nil || l.Disk == nil {
		return nil
	}

	// Disk value must be provided, as it is required when creating an instance.
	nDisk, err := units.RAMInBytes(*n.Disk)
	if err != nil {
		return err
	}
	lDisk, err := units.RAMInBytes(*l.Disk)
	if err != nil {
		return err
	}

	// Reject shrinking disk
	if nDisk < lDisk {
		return fmt.Errorf("field `disk`: shrinking the disk (%v --> %v) is not supported", *l.Disk, *n.Disk)
	}

	return nil
}
