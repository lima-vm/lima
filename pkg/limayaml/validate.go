package limayaml

import (
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/localpathutil"
	"github.com/AkihiroSuda/lima/pkg/qemu/qemuconst"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func Validate(y LimaYAML) error {
	switch y.Arch {
	case X8664, AARCH64:
	default:
		return errors.Errorf("field `arch` must be %q or %q , got %q", X8664, AARCH64, y.Arch)
	}

	if len(y.Images) == 0 {
		return errors.New("field `images` must be set")
	}
	for i, f := range y.Images {
		if !strings.Contains(f.Location, "://") {
			if _, err := localpathutil.Expand(f.Location); err != nil {
				return errors.Wrapf(err, "field `images[%d].location` refers to an invalid local file path: %q",
					i, f.Location)
			}
			// f.Location does NOT need to be accessible, so we do NOT check os.Stat(f.Location)
		}
		switch f.Arch {
		case X8664, AARCH64:
		default:
			return errors.Errorf("field `images.arch` must be %q or %q, got %q", X8664, AARCH64, f.Arch)
		}
		if f.Digest != "" {
			if !f.Digest.Algorithm().Available() {
				return errors.Errorf("field `images[%d].digest` refers to an unavailable digest algorithm", i)
			}
			if err := f.Digest.Validate(); err != nil {
				return errors.Wrapf(err, "field `images[%d].digest` is invalid: %s", i, f.Digest.String())
			}
		}
	}

	if y.CPUs == 0 {
		return errors.New("field `cpus` must be set")
	}

	if _, err := units.RAMInBytes(y.Memory); err != nil {
		return errors.Wrapf(err, "field `memory` has an invalid value")
	}

	if _, err := units.RAMInBytes(y.Disk); err != nil {
		return errors.Wrapf(err, "field `memory` has an invalid value")
	}

	u, err := user.Current()
	if err != nil {
		return errors.Wrap(err, "internal error (not an error of YAML)")
	}
	// reservedHome is the home directory defined in "cidata.iso:/user-data"
	reservedHome := fmt.Sprintf("/home/%s.linux", u.Username)

	for i, f := range y.Mounts {
		if !filepath.IsAbs(f.Location) && !strings.HasPrefix(f.Location, "~") {
			return errors.Errorf("field `mounts[%d].location` must be an absolute path, got %q",
				i, f.Location)
		}
		loc, err := localpathutil.Expand(f.Location)
		if err != nil {
			return errors.Wrapf(err, "field `mounts[%d].location` refers to an unexpandable path: %q",
				i, f.Location)
		}
		switch loc {
		case "/", "/bin", "/dev", "/etc", "/home", "/opt", "/sbin", "/tmp", "/usr", "/var":
			return errors.Errorf("field `mounts[%d].location` must not be a system path such as /etc or /usr", i)
		case reservedHome:
			return errors.Errorf("field `mounts[%d].location` is internally reserved", i)
		}

		st, err := os.Stat(loc)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return errors.Wrapf(err, "field `mounts[%d].location` refers to an inaccessible path: %q",
					i, f.Location)
			}
		} else if !st.IsDir() {
			return errors.Wrapf(err, "field `mounts[%d].location` refers to a non-directory path: %q",
				i, f.Location)
		}
	}

	if err := validatePort("ssh.localPort", y.SSH.LocalPort); err != nil {
		return err
	}

	// y.Firmware.LegacyBIOS is ignored for aarch64, but not a fatal error.

	for i, p := range y.Provision {
		switch p.Mode {
		case ProvisionModeSystem, ProvisionModeUser:
		default:
			return errors.Errorf("field `provision[%d].mode` must be either %q or %q",
				i, ProvisionModeSystem, ProvisionModeUser)
		}
	}
	for i, p := range y.Probes {
		switch p.Mode {
		case ProbeModeReadiness:
		default:
			return errors.Errorf("field `probe[%d].mode` can only be %q",
				i, ProbeModeReadiness)
		}
	}
	for i, rule := range y.PortForwards {
		field := fmt.Sprintf("portForwards[%d]", i)
		if rule.GuestPort != 0 {
			if rule.GuestPort != rule.GuestPortRange[0] {
				return errors.Errorf("field `%s.guestPort` must match field `%s.guestPortRange[0]`", field, field)
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".guestPort", rule.GuestPort); err != nil {
				return err
			}
		}
		if rule.HostPort != 0 {
			if rule.HostPort != rule.HostPortRange[0] {
				return errors.Errorf("field `%s.hostPort` must match field `%s.hostPortRange[0]`", field, field)
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
			return errors.Errorf("field `%s.guestPortRange[1]` must be greater than or equal to field `%s.guestPortRange[0]`", field, field)
		}
		if rule.HostPortRange[0] > rule.HostPortRange[1] {
			return errors.Errorf("field `%s.hostPortRange[1]` must be greater than or equal to field `%s.hostPortRange[0]`", field, field)
		}
		if rule.GuestPortRange[1]-rule.GuestPortRange[0] != rule.HostPortRange[1]-rule.HostPortRange[0] {
			return errors.Errorf("field `%s.hostPortRange` must specify the same number of ports as field `%s.guestPortRange`", field, field)
		}
		if rule.Proto != TCP {
			return errors.Errorf("field `%s.proto` must be %q", field, TCP)
		}
		// Not validating that the various GuestPortRanges and HostPortRanges are not overlapping. Rules will be
		// processed sequentially and the first matching rule for a guest port determines forwarding behavior.
	}

	if err := validateNetwork(y.Network); err != nil {
		return err
	}

	return nil
}

func validateNetwork(yNetwork Network) error {
	networkName := make(map[string]int)
	for i, vde := range yNetwork.VDE {
		field := fmt.Sprintf("network.vde[%d]", i)
		if vde.URL == "" {
			return errors.Errorf("field `%s.url` must not be empty", field)
		}
		// The field is called VDE.URL in anticipation of QEMU upgrading VDE2 to VDEplug4,
		// but right now the only valid value on macOS is a path to the vde_switch socket directory,
		// optionally with vde:// prefix.
		if !strings.Contains(vde.URL, "://") || strings.HasPrefix(vde.URL, "vde://") {
			vdeSwitch := strings.TrimPrefix(vde.URL, "vde://")
			fi, err := os.Stat(vdeSwitch)
			if err != nil {
				return errors.Wrapf(err, "field `%s.url` %q failed stat", field, vdeSwitch)
			}
			if !fi.IsDir() {
				return errors.Wrapf(err, "field `%s.url` %q is not a directory", field, vdeSwitch)
			}
			ctlSocket := filepath.Join(vdeSwitch, "ctl")
			fi, err = os.Stat(ctlSocket)
			if err != nil {
				return errors.Wrapf(err, "field `%s.url` control socket %q failed stat", field, ctlSocket)
			}
			if fi.Mode()&os.ModeSocket == 0 {
				return errors.Errorf("field `%s.url` file %q is not a UNIX socket", field, ctlSocket)
			}
		} else if runtime.GOOS != "linux" {
			logrus.Warnf("field `%s.url` is unlikely to work for %s (unless libvdeplug4 has been ported to %s and is installed)",
				field, runtime.GOOS, runtime.GOOS)
		}
		if vde.MACAddress != "" {
			hw, err := net.ParseMAC(vde.MACAddress)
			if err != nil {
				return errors.Wrap(err, "field `vmnet.mac` invalid")
			}
			if len(hw) != 6 {
				return errors.Errorf("field `%s.macAddress` must be a 48 bit (6 bytes) MAC address; actual length of %q is %d bytes", field, vde.MACAddress, len(hw))
			}
		}
		// FillDefault() will make sure that vde.Name is not the empty string
		if len(vde.Name) >= 16 {
			return errors.Errorf("field `%s.name` must be less than 16 bytes, but is %d bytes: %q", field, len(vde.Name), vde.Name)
		}
		if strings.ContainsAny(vde.Name, " \t\n/") {
			return errors.Errorf("field `%s.name` must not contain whitespace or slashes", field)
		}
		if vde.Name == qemuconst.SlirpNICName {
			return errors.Errorf("field `%s.name` must not be set to %q because it is reserved for slirp", field, qemuconst.SlirpNICName)
		}
		if prev, ok := networkName[vde.Name]; ok {
			return errors.Errorf("field `%s.name` value %q has already been used by field `network.vde[%d].name`", field, vde.Name, prev)
		}
		networkName[vde.Name] = i
	}
	return nil
}

func validatePort(field string, port int) error {
	switch {
	case port < 0:
		return errors.Errorf("field `%s` must be > 0", field)
	case port == 0:
		return errors.Errorf("field `%s` must be set", field)
	case port == 22:
		return errors.Errorf("field `%s` must not be 22", field)
	case port > 65535:
		return errors.Errorf("field `%s` must be < 65536", field)
	}
	return nil
}
