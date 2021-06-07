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

	switch {
	case y.SSH.LocalPort < 0:
		return errors.New("field `ssh.localPort` must be > 0")
	case y.SSH.LocalPort == 0:
		return errors.New("field `ssh.localPort` must be set, e.g, 60022 (FIXME: support automatic port assignment)")
	case y.SSH.LocalPort == 22:
		return errors.New("field `ssh.localPort` must not be 22")
	case y.SSH.LocalPort > 65535:
		return errors.New("field `ssh.localPort` must be < 65535")
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

	return nil
}
