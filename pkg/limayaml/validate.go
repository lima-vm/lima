// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package limayaml

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driverutil"
	"github.com/lima-vm/lima/v2/pkg/identifiers"
	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/localpathutil"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/version"
	"github.com/lima-vm/lima/v2/pkg/version/versionutil"
)

func Validate(y *limatype.LimaYAML, warn bool) error {
	var errs error

	if len(y.Base) > 0 {
		errs = errors.Join(errs, errors.New("field `base` must be empty for YAML validation"))
	}

	if y.MinimumLimaVersion != nil {
		if _, err := versionutil.Parse(*y.MinimumLimaVersion); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `minimumLimaVersion` must be a semvar value, got %q: %w", *y.MinimumLimaVersion, err))
		}
		// Unparsable version.Version (like commit hashes or "<unknown>") is treated as "latest/greatest"
		// and will pass all version comparisons, allowing development builds to work.
		if !versionutil.GreaterEqual(version.Version, *y.MinimumLimaVersion) {
			errs = errors.Join(errs, fmt.Errorf("template requires Lima version %q; this is only %q", *y.MinimumLimaVersion, version.Version))
		}
	}

	switch *y.OS {
	case limatype.LINUX, limatype.DARWIN, limatype.FREEBSD:
	default:
		errs = errors.Join(errs, fmt.Errorf("field `os` must be one of %q; got %q", limatype.OSTypes, *y.OS))
	}
	if !slices.Contains(limatype.ArchTypes, *y.Arch) {
		errs = errors.Join(errs, fmt.Errorf("field `arch` must be one of %v; got %q", limatype.ArchTypes, *y.Arch))
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

	if *y.CPUs == 0 {
		errs = errors.Join(errs, errors.New("field `cpus` must be set"))
	}

	if y.Memory == nil {
		errs = errors.Join(errs, errors.New("field `memory` must be set"))
	} else if _, err := units.RAMInBytes(*y.Memory); err != nil {
		errs = errors.Join(errs, fmt.Errorf("field `memory` has an invalid value: %w", err))
	}

	if y.Disk == nil {
		errs = errors.Join(errs, errors.New("field `disk` must be set"))
	} else if _, err := units.RAMInBytes(*y.Disk); err != nil {
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

	if y.MountType != nil {
		switch *y.MountType {
		case limatype.REVSSHFS, limatype.NINEP, limatype.VIRTIOFS, limatype.WSLMount:
		default:
			errs = errors.Join(errs, fmt.Errorf("field `mountType` must be %q or %q or %q, or %q, got %q", limatype.REVSSHFS, limatype.NINEP, limatype.VIRTIOFS, limatype.WSLMount, *y.MountType))
		}

		if slices.Contains(y.MountTypesUnsupported, *y.MountType) {
			errs = errors.Join(errs, fmt.Errorf("field `mountType` must not be one of %v (`mountTypesUnsupported`), got %q", y.MountTypesUnsupported, *y.MountType))
		}
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
		case limatype.ProvisionModeSystem, limatype.ProvisionModeUser, limatype.ProvisionModeBoot, limatype.ProvisionModeData, limatype.ProvisionModeDependency, limatype.ProvisionModeAnsible, limatype.ProvisionModeYQ:
		default:
			errs = errors.Join(errs, fmt.Errorf("field `provision[%d].mode` must one of %q, %q, %q, %q, %q, %q, or %q",
				i, limatype.ProvisionModeSystem, limatype.ProvisionModeUser, limatype.ProvisionModeBoot, limatype.ProvisionModeData, limatype.ProvisionModeDependency, limatype.ProvisionModeAnsible, limatype.ProvisionModeYQ))
		}
		if p.Mode != limatype.ProvisionModeDependency && p.SkipDefaultDependencyResolution != nil {
			errs = errors.Join(errs, fmt.Errorf("field `provision[%d].mode` cannot set skipDefaultDependencyResolution, only valid on scripts of type %q",
				i, limatype.ProvisionModeDependency))
		}

		// This can lead to fatal Panic if p.Path is nil, better to return an error here
		switch p.Mode {
		case limatype.ProvisionModeData, limatype.ProvisionModeYQ:
			if p.Path == nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].path` must not be empty when mode is %q", i, p.Mode))
				return errs
			}
			if !path.IsAbs(*p.Path) {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].path` must be an absolute path", i))
			}
			if p.Mode == limatype.ProvisionModeData && p.Content == nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].content` must not be empty when mode is %q", i, p.Mode))
			}
			if p.Mode == limatype.ProvisionModeYQ && p.Expression == nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].expression` must not be empty when mode is %q", i, p.Mode))
			}
			// FillDefaults makes sure that p.Permissions is not nil
			if _, err := strconv.ParseInt(*p.Permissions, 8, 64); err != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].permissions` must be an octal number: %w", i, err))
			}
		default:
			if (p.Script == nil || *p.Script == "") && p.Mode != limatype.ProvisionModeAnsible {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].script` must not be empty", i))
			}
			if p.Content != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].content` can only be set when mode is %q", i, limatype.ProvisionModeData))
			}
			if p.Overwrite != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].overwrite` can only be set when mode is %q", i, limatype.ProvisionModeData))
			}
			if p.Owner != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].owner` can only be set when mode is %q", i, limatype.ProvisionModeData))
			}
			if p.Path != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].path` can only be set when mode is %q, or %q", i, limatype.ProvisionModeData, limatype.ProvisionModeYQ))
			}
			if p.Permissions != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].permissions` can only be set when mode is %q, or %q", i, limatype.ProvisionModeData, limatype.ProvisionModeYQ))
			}
			if p.Format != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].format` can only be set when mode is %q", i, limatype.ProvisionModeYQ))
			}
		}
		if p.Playbook != "" {
			if p.Mode != limatype.ProvisionModeAnsible {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].playbook can only be set when mode is %q", i, limatype.ProvisionModeAnsible))
			}
			if p.Script != nil && *p.Script != "" {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].script must be empty if playbook is set", i))
			}
			playbook := p.Playbook
			if _, err := os.Stat(playbook); err != nil {
				errs = errors.Join(errs, fmt.Errorf("field `provision[%d].playbook` refers to an inaccessible path: %q: %w", i, playbook, err))
			}
			logrus.Warnf("provision mode %q is deprecated, use `ansible-playbook %q` instead", limatype.ProvisionModeAnsible, playbook)
		}
		if p.Script != nil {
			if strings.Contains(*p.Script, "LIMA_CIDATA") {
				logrus.Warn("provisioning scripts should not reference the LIMA_CIDATA variables")
			}
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
		if p.Script != nil && !strings.HasPrefix(*p.Script, "#!") {
			errs = errors.Join(errs, fmt.Errorf("field `probe[%d].script` must start with a '#!' line", i))
		}
		switch p.Mode {
		case limatype.ProbeModeReadiness:
		default:
			errs = errors.Join(errs, fmt.Errorf("field `probe[%d].mode` can only be %q", i, limatype.ProbeModeReadiness))
		}
	}
	for i, rule := range y.PortForwards {
		field := fmt.Sprintf("portForwards[%d]", i)
		if *rule.GuestIPMustBeZero && !rule.GuestIP.Equal(net.IPv4zero) {
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
			if rule.HostSocket == "" {
				if err := validatePort(fmt.Sprintf("%s.hostPortRange[%d]", field, j), rule.HostPortRange[j]); err != nil {
					errs = errors.Join(errs, err)
				}
			}
		}
		if rule.GuestPortRange[0] > rule.GuestPortRange[1] {
			errs = errors.Join(errs, fmt.Errorf("field `%s.guestPortRange[1]` must be greater than or equal to field `%s.guestPortRange[0]`", field, field))
		}
		if rule.HostPortRange[0] > rule.HostPortRange[1] {
			errs = errors.Join(errs, fmt.Errorf("field `%s.hostPortRange[1]` must be greater than or equal to field `%s.hostPortRange[0]`", field, field))
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
		} else if rule.GuestPortRange[1]-rule.GuestPortRange[0] != rule.HostPortRange[1]-rule.HostPortRange[0] {
			errs = errors.Join(errs, fmt.Errorf("field `%s.hostPortRange` must specify the same number of ports as field `%s.guestPortRange`", field, field))
		}

		if len(rule.HostSocket) >= osutil.UnixPathMax {
			errs = errors.Join(errs, fmt.Errorf("field `%s.hostSocket` must be less than UNIX_PATH_MAX=%d characters, but is %d",
				field, osutil.UnixPathMax, len(rule.HostSocket)))
		}
		switch rule.Proto {
		case limatype.ProtoTCP, limatype.ProtoUDP, limatype.ProtoAny:
		default:
			errs = errors.Join(errs, fmt.Errorf("field `%s.proto` must be %q, %q, or %q", field, limatype.ProtoTCP, limatype.ProtoUDP, limatype.ProtoAny))
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
	if y.Plain != nil && *y.Plain {
		const portRangeWarnThreshold = 10
		for i, rule := range y.PortForwards {
			guestRange := rule.GuestPortRange[1] - rule.GuestPortRange[0] + 1
			hostRange := rule.HostPortRange[1] - rule.HostPortRange[0] + 1
			if guestRange > portRangeWarnThreshold || hostRange > portRangeWarnThreshold {
				logrus.Warnf("[plain mode] portForwards[%d] covers a range of more than %d ports (guest: %d, host: %d). All ports will be forwarded unconditionally, which may be inefficient.", i, portRangeWarnThreshold, guestRange, hostRange)
			}
		}
	}

	errs = errors.Join(errs, validateMemoryBalloon(y))
	errs = errors.Join(errs, validateAutoPause(y))

	return errs
}

func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

func validateMemoryBalloon(y *limatype.LimaYAML) error {
	if y.VMOpts == nil {
		return nil
	}
	var vzOpts limatype.VZOpts
	if err := Convert(y.VMOpts[limatype.VZ], &vzOpts, "vmOpts.vz"); err != nil {
		return nil // No VZ opts to validate.
	}
	balloon := vzOpts.MemoryBalloon

	// If balloon is not enabled, skip all validation.
	if balloon.Enabled == nil || !*balloon.Enabled {
		return nil
	}

	var errs error
	const field = "vmOpts.vz.memoryBalloon"

	// Rule 1: balloon requires vmType "vz".
	if y.VMType != nil && *y.VMType != limatype.VZ {
		errs = errors.Join(errs, fmt.Errorf("field `%s` requires vmType %q, got %q", field, limatype.VZ, *y.VMType))
	}

	// Parse min and idleTarget for comparison.
	var minBytes, idleTargetBytes int64
	if balloon.Min != nil {
		var err error
		minBytes, err = units.RAMInBytes(*balloon.Min)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.min` must be a valid byte size: %w", field, err))
		}
		if minBytes <= 0 {
			errs = errors.Join(errs, fmt.Errorf("field `%s.min` must be greater than 0", field))
		}
	}
	if balloon.IdleTarget != nil {
		var err error
		idleTargetBytes, err = units.RAMInBytes(*balloon.IdleTarget)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.idleTarget` must be a valid byte size: %w", field, err))
		}
	}

	// Rule 2: min < idleTarget.
	if minBytes > 0 && idleTargetBytes > 0 && minBytes >= idleTargetBytes {
		errs = errors.Join(errs, fmt.Errorf("field `%s.min` must be less than `idleTarget`", field))
	}

	// Rule 2b: idleTarget must not exceed VM memory.
	if y.Memory != nil && idleTargetBytes > 0 {
		memoryBytes, memErr := units.RAMInBytes(*y.Memory)
		if memErr == nil && idleTargetBytes > memoryBytes {
			errs = errors.Join(errs, fmt.Errorf("field `%s.idleTarget` must not exceed `memory` (%s)", field, *y.Memory))
		}
	}

	// Rule 3: step percents 1-100.
	if balloon.GrowStepPercent != nil && (*balloon.GrowStepPercent < 1 || *balloon.GrowStepPercent > 100) {
		errs = errors.Join(errs, fmt.Errorf("field `%s.growStepPercent` must be between 1 and 100", field))
	}
	if balloon.ShrinkStepPercent != nil && (*balloon.ShrinkStepPercent < 1 || *balloon.ShrinkStepPercent > 100) {
		errs = errors.Join(errs, fmt.Errorf("field `%s.shrinkStepPercent` must be between 1 and 100", field))
	}

	// Rule 4: thresholds 0.0-1.0 (NaN check required because NaN fails both < and > comparisons).
	if balloon.HighPressureThreshold != nil &&
		(math.IsNaN(*balloon.HighPressureThreshold) || *balloon.HighPressureThreshold < 0.0 || *balloon.HighPressureThreshold > 1.0) {
		errs = errors.Join(errs, fmt.Errorf("field `%s.highPressureThreshold` must be between 0.0 and 1.0", field))
	}
	if balloon.LowPressureThreshold != nil &&
		(math.IsNaN(*balloon.LowPressureThreshold) || *balloon.LowPressureThreshold < 0.0 || *balloon.LowPressureThreshold > 1.0) {
		errs = errors.Join(errs, fmt.Errorf("field `%s.lowPressureThreshold` must be between 0.0 and 1.0", field))
	}

	// Rule 5: high > low.
	if balloon.HighPressureThreshold != nil && balloon.LowPressureThreshold != nil &&
		*balloon.HighPressureThreshold <= *balloon.LowPressureThreshold {
		errs = errors.Join(errs, fmt.Errorf("field `%s.highPressureThreshold` (%.2f) must be greater than `lowPressureThreshold` (%.2f)",
			field, *balloon.HighPressureThreshold, *balloon.LowPressureThreshold))
	}

	// Rule 6: durations must be parseable.
	if balloon.Cooldown != nil {
		if _, err := parseDuration(*balloon.Cooldown); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.cooldown` must be a valid duration: %w", field, err))
		}
	}
	if balloon.IdleGracePeriod != nil {
		if _, err := parseDuration(*balloon.IdleGracePeriod); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.idleGracePeriod` must be a valid duration: %w", field, err))
		}
	}
	if balloon.SettleWindow != nil {
		if _, err := parseDuration(*balloon.SettleWindow); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.settleWindow` must be a valid duration: %w", field, err))
		}
	}

	// Rule 7: byte sizes must be parseable.
	if balloon.MaxSwapInPerSec != nil {
		if _, err := units.RAMInBytes(*balloon.MaxSwapInPerSec); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.maxSwapInPerSec` must be a valid byte size: %w", field, err))
		}
	}
	if balloon.MaxSwapOutPerSec != nil {
		if _, err := units.RAMInBytes(*balloon.MaxSwapOutPerSec); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.maxSwapOutPerSec` must be a valid byte size: %w", field, err))
		}
	}
	if balloon.ShrinkReserveBytes != nil {
		if _, err := units.RAMInBytes(*balloon.ShrinkReserveBytes); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.shrinkReserveBytes` must be a valid byte size: %w", field, err))
		}
	}
	if balloon.MaxContainerIO != nil {
		if _, err := units.RAMInBytes(*balloon.MaxContainerIO); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.maxContainerIO` must be a valid byte size: %w", field, err))
		}
	}

	// Rule 8: maxContainerCPU > 0.
	if balloon.MaxContainerCPU != nil && *balloon.MaxContainerCPU <= 0.0 {
		errs = errors.Join(errs, fmt.Errorf("field `%s.maxContainerCPU` must be greater than 0.0", field))
	}

	return errs
}

func validateAutoPause(y *limatype.LimaYAML) error {
	if y.VMOpts == nil {
		return nil
	}
	var vzOpts limatype.VZOpts
	if err := Convert(y.VMOpts[limatype.VZ], &vzOpts, "vmOpts.vz"); err != nil {
		return nil // No VZ opts to validate.
	}
	ap := vzOpts.AutoPause

	// If auto-pause is not enabled, skip all validation.
	if ap.Enabled == nil || !*ap.Enabled {
		return nil
	}

	var errs error
	const field = "vmOpts.vz.autoPause"

	// Rule 1: autoPause requires vmType "vz".
	if y.VMType != nil && *y.VMType != limatype.VZ {
		errs = errors.Join(errs, fmt.Errorf("field `%s` requires vmType %q, got %q", field, limatype.VZ, *y.VMType))
	}

	// Rule 2: idleTimeout must be a valid duration >= 1m.
	if ap.IdleTimeout != nil {
		d, err := parseDuration(*ap.IdleTimeout)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.idleTimeout` must be a valid duration: %w", field, err))
		} else if d < time.Minute {
			errs = errors.Join(errs, fmt.Errorf("field `%s.idleTimeout` must be at least 1m, got %s", field, d))
		}
	}

	// Rule 3: resumeTimeout must be a valid duration >= 5s.
	if ap.ResumeTimeout != nil {
		d, err := parseDuration(*ap.ResumeTimeout)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.resumeTimeout` must be a valid duration: %w", field, err))
		} else if d < 5*time.Second {
			errs = errors.Join(errs, fmt.Errorf("field `%s.resumeTimeout` must be at least 5s, got %s", field, d))
		}
	}

	// Rule 4: autoPause requires memoryBalloon to be enabled.
	balloon := vzOpts.MemoryBalloon
	if balloon.Enabled == nil || !*balloon.Enabled {
		errs = errors.Join(errs, fmt.Errorf("field `%s` requires `vmOpts.vz.memoryBalloon.enabled` to be true", field))
	}

	// Rule 5: containerCPUThreshold must be in range [0.0, 100.0] if specified.
	if ap.IdleSignals.ContainerCPUThreshold != nil {
		threshold := *ap.IdleSignals.ContainerCPUThreshold
		if math.IsNaN(threshold) || threshold < 0.0 || threshold > 100.0 {
			errs = errors.Join(errs, fmt.Errorf(
				"field `%s.idleSignals.containerCPUThreshold` must be between 0.0 and 100.0, got %g",
				field, threshold))
		}
	}

	// Rule 6: warn if containerCPU is disabled but containerCPUThreshold is set.
	if ap.IdleSignals.ContainerCPU != nil && !*ap.IdleSignals.ContainerCPU &&
		ap.IdleSignals.ContainerCPUThreshold != nil {
		logrus.Warnf("field `%s.idleSignals.containerCPUThreshold` is set but "+
			"`containerCPU` is disabled; threshold will be ignored", field)
	}

	return errs
}

func validateFileObject(f limatype.File, fieldName string) error {
	var errs error
	if !strings.Contains(f.Location, "://") {
		if _, err := localpathutil.Expand(f.Location); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.location` refers to an invalid local file path: %q: %w", fieldName, f.Location, err))
		}
		// f.Location does NOT need to be accessible, so we do NOT check os.Stat(f.Location)
	}
	if !slices.Contains(limatype.ArchTypes, f.Arch) {
		errs = errors.Join(errs, fmt.Errorf("field `arch` must be one of %v; got %q", limatype.ArchTypes, f.Arch))
	}
	if f.Digest != "" {
		if err := f.Digest.Validate(); err != nil {
			errs = errors.Join(errs, fmt.Errorf("field `%s.digest` is invalid: %s: %w", fieldName, f.Digest.String(), err))
		}
	}
	return errs
}

func validateNetwork(y *limatype.LimaYAML) error {
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
func validateParamIsUsed(y *limatype.LimaYAML) error {
	for key := range y.Param {
		re, err := regexp.Compile(`{{[^}]*\.Param\.` + key + `[^}]*}}|\bPARAM_` + key + `\b`)
		if err != nil {
			return fmt.Errorf("field to compile regexp for key %q: %w", key, err)
		}
		keyIsUsed := false
		for _, p := range y.Provision {
			for _, ptr := range []*string{p.Script, p.Content, p.Expression, p.Owner, p.Path, p.Permissions} {
				if ptr != nil && re.MatchString(*ptr) {
					keyIsUsed = true
					break
				}
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
			if p.Script != nil && re.MatchString(*p.Script) {
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

func warnExperimental(y *limatype.LimaYAML) {
	if *y.MountType == limatype.VIRTIOFS && runtime.GOOS == "linux" {
		logrus.Warn("`mountType: virtiofs` on Linux is experimental")
	}
	switch *y.Arch {
	case limatype.RISCV64, limatype.ARMV7L, limatype.S390X, limatype.PPC64LE:
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
func ValidateAgainstLatestConfig(ctx context.Context, yNew, yLatest []byte) error {
	var n limatype.LimaYAML
	var errs error

	// Load the latest YAML and fill in defaults
	l, err := LoadWithWarnings(ctx, yLatest, "")
	if err != nil {
		errs = errors.Join(errs, err)
	}
	if err := driverutil.ResolveVMType(ctx, l, ""); err != nil {
		errs = errors.Join(errs, fmt.Errorf("failed to resolve vm for %q: %w", "", err))
	}
	if err := Unmarshal(yNew, &n, "Unmarshal new YAML bytes"); err != nil {
		errs = errors.Join(errs, err)
	}

	// Handle editing the template without a disk value
	if n.Disk == nil || l.Disk == nil {
		return errs
	}

	// Disk value must be provided, as it is required when creating an instance.
	nDisk, err := units.RAMInBytes(*n.Disk)
	if err != nil {
		errs = errors.Join(errs, err)
	}
	lDisk, err := units.RAMInBytes(*l.Disk)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	// Reject shrinking disk
	if nDisk < lDisk {
		errs = errors.Join(errs, fmt.Errorf("field `disk`: shrinking the disk (%v --> %v) is not supported", *l.Disk, *n.Disk))
	}

	return errs
}
