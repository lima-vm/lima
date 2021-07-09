package limayaml

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/AkihiroSuda/lima/pkg/localpathutil"
	"github.com/docker/go-units"
	"github.com/pkg/errors"
)

func Validate(y LimaYAML) error {
	FillDefault(&y)
	return ValidateRaw(y)
}

func ValidateRaw(y LimaYAML) error {
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
	for i, port := range y.Ports {
		field := fmt.Sprintf("ports[%d]", i)
		if port.GuestIP != "127.0.0.1" {
			return errors.Errorf("field `%s.guestIP` must be \"127.0.0.1\"", field)
		}
		if port.HostIP != "127.0.0.1" && port.HostIP != "0.0.0.0" {
			return errors.Errorf("field `%s.hostIP` must be either \"127.0.0.1\" or \"0.0.0.0\"", field)
		}
		if port.GuestPort != 0 {
			if port.GuestPort != port.GuestPortRange[0] {
				return errors.Errorf("field `%s.guestPort` must match field `%s.guestPortRange[0]`", field, field)
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".guestPort", port.GuestPort); err != nil {
				return err
			}
		}
		if port.HostPort != 0 {
			if port.HostPort != port.HostPortRange[0] {
				return errors.Errorf("field `%s.hostPort` must match field `%s.hostPortRange[0]`", field, field)
			}
			// redundant validation to make sure the error contains the correct field name
			if err := validatePort(field+".hostPort", port.HostPort); err != nil {
				return err
			}
		}
		for j := 0; j < 2; j++ {
			if err := validatePort(fmt.Sprintf("%s.guestPortRange[%d]", field, j), port.GuestPortRange[j]); err != nil {
				return err
			}
			if err := validatePort(fmt.Sprintf("%s.hostPortRange[%d]", field, j), port.HostPortRange[j]); err != nil {
				return err
			}
		}
		if port.GuestPortRange[0] > port.GuestPortRange[1] {
			return errors.Errorf("field `%s.guestPortRange[1]` must be greater than or equal to field `%s.guestPortRange[0]`", field, field)
		}
		if port.HostPortRange[0] > port.HostPortRange[1] {
			return errors.Errorf("field `%s.hostPortRange[1]` must be greater than or equal to field `%s.hostPortRange[0]`", field, field)
		}
		if port.GuestPortRange[1] - port.GuestPortRange[0] != port.HostPortRange[1] - port.HostPortRange[0] {
			return errors.Errorf("field `%s.hostPortRange` must specify the same number of ports as field `%s.guestPortRange`", field, field)
		}
		if port.Proto != TCP {
			return errors.Errorf("field `%s.proto` must be %q", field, TCP)
		}
		// Not validating that the various GuestPortRanges and HostPortRanges are not overlapping. Rules will be
		// processed sequentially and the first matching rule for a guest port determines forwarding behavior.
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
