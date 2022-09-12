package networks

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
)

const (
	VDESwitch   = "vde_switch" // Deprecated
	VDEVMNet    = "vde_vmnet"  // Deprecated
	SocketVMNet = "socket_vmnet"
)

// Commands in `sudoers` cannot use quotes, so all arguments are printed via "%s"
// and not "%q". config.Paths.* entries must not include any whitespace!

func (config *NetworksConfig) Check(name string) error {
	if _, ok := config.Networks[name]; ok {
		return nil
	}
	return fmt.Errorf("network %q is not defined", name)
}

// DaemonPath returns the daemon path.
func (config *NetworksConfig) DaemonPath(daemon string) (string, error) {
	switch daemon {
	case VDESwitch:
		return config.Paths.VDESwitch, nil
	case VDEVMNet:
		return config.Paths.VDEVMNet, nil
	case SocketVMNet:
		return config.Paths.SocketVMNet, nil
	default:
		return "", fmt.Errorf("unknown daemon type %q", daemon)
	}
}

// IsDaemonInstalled checks whether the daemon is installed.
func (config *NetworksConfig) IsDaemonInstalled(daemon string) (bool, error) {
	p, err := config.DaemonPath(daemon)
	if err != nil {
		return false, err
	}
	if p == "" {
		return false, nil
	}
	if _, err := exec.LookPath(p); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Sock returns a socket_vmnet socket.
func (config *NetworksConfig) Sock(name string) string {
	return filepath.Join(config.Paths.VarRun, fmt.Sprintf("socket_vmnet.%s", name))
}

// VDESock returns a vde socket.
//
// Deprecated. Use Sock.
func (config *NetworksConfig) VDESock(name string) string {
	return filepath.Join(config.Paths.VarRun, fmt.Sprintf("%s.ctl", name))
}

func (config *NetworksConfig) PIDFile(name, daemon string) string {
	daemonTrimmed := strings.TrimPrefix(daemon, "vde_") // for compatibility
	return filepath.Join(config.Paths.VarRun, fmt.Sprintf("%s_%s.pid", name, daemonTrimmed))
}

func (config *NetworksConfig) LogFile(name, daemon, stream string) string {
	networksDir, _ := dirnames.LimaNetworksDir()
	daemonTrimmed := strings.TrimPrefix(daemon, "vde_") // for compatibility
	return filepath.Join(networksDir, fmt.Sprintf("%s_%s.%s.log", name, daemonTrimmed, stream))
}

func (config *NetworksConfig) User(daemon string) (osutil.User, error) {
	if ok, _ := config.IsDaemonInstalled(daemon); !ok {
		daemonPath, _ := config.DaemonPath(daemon)
		return osutil.User{}, fmt.Errorf("daemon %q (path=%q) is not available", daemon, daemonPath)
	}
	switch daemon {
	case VDESwitch:
		user, err := osutil.LookupUser("daemon")
		if err != nil {
			return user, err
		}
		group, err := osutil.LookupGroup(config.Group)
		user.Group = group.Name
		user.Gid = group.Gid
		return user, err
	case VDEVMNet, SocketVMNet:
		return osutil.LookupUser("root")
	}
	return osutil.User{}, fmt.Errorf("daemon %q not defined", daemon)
}

func (config *NetworksConfig) MkdirCmd() string {
	return fmt.Sprintf("/bin/mkdir -m 775 -p %s", config.Paths.VarRun)
}

func (config *NetworksConfig) StartCmd(name, daemon string) string {
	if ok, _ := config.IsDaemonInstalled(daemon); !ok {
		panic(fmt.Errorf("daemon %q is not available", daemon))
	}
	var cmd string
	switch daemon {
	case VDESwitch:
		if config.Paths.VDESwitch == "" {
			panic("config.Paths.VDESwitch is empty")
		}
		cmd = fmt.Sprintf("%s --pidfile=%s --sock=%s --group=%s --dirmode=0770 --nostdin",
			config.Paths.VDESwitch, config.PIDFile(name, VDESwitch), config.VDESock(name), config.Group)
	case VDEVMNet:
		nw := config.Networks[name]
		if config.Paths.VDEVMNet == "" {
			panic("config.Paths.VDEVMNet is empty")
		}
		cmd = fmt.Sprintf("%s --pidfile=%s --vde-group=%s --vmnet-mode=%s",
			config.Paths.VDEVMNet, config.PIDFile(name, VDEVMNet), config.Group, nw.Mode)
		switch nw.Mode {
		case ModeBridged:
			cmd += fmt.Sprintf(" --vmnet-interface=%s", nw.Interface)
		case ModeHost, ModeShared:
			cmd += fmt.Sprintf(" --vmnet-gateway=%s --vmnet-dhcp-end=%s --vmnet-mask=%s",
				nw.Gateway, nw.DHCPEnd, nw.NetMask)
		}
		cmd += " " + config.VDESock(name)
	case SocketVMNet:
		nw := config.Networks[name]
		if config.Paths.SocketVMNet == "" {
			panic("config.Paths.SocketVMNet is empty")
		}
		cmd = fmt.Sprintf("%s --pidfile=%s --socket-group=%s --vmnet-mode=%s",
			config.Paths.SocketVMNet, config.PIDFile(name, SocketVMNet), config.Group, nw.Mode)
		switch nw.Mode {
		case ModeBridged:
			cmd += fmt.Sprintf(" --vmnet-interface=%s", nw.Interface)
		case ModeHost, ModeShared:
			cmd += fmt.Sprintf(" --vmnet-gateway=%s --vmnet-dhcp-end=%s --vmnet-mask=%s",
				nw.Gateway, nw.DHCPEnd, nw.NetMask)
		}
		cmd += " " + config.Sock(name)
	}
	return cmd
}

func (config *NetworksConfig) StopCmd(name, daemon string) string {
	return fmt.Sprintf("/usr/bin/pkill -F %s", config.PIDFile(name, daemon))
}
