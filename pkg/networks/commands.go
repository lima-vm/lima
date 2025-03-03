// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"errors"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"

	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store/dirnames"
)

const (
	SocketVMNet = "socket_vmnet"
)

// Commands in `sudoers` cannot use quotes, so all arguments are printed via "%s"
// and not "%q". cfg.Paths.* entries must not include any whitespace!

func (c *Config) Check(name string) error {
	if _, ok := c.Networks[name]; ok {
		return nil
	}
	return fmt.Errorf("network %q is not defined", name)
}

// Usernet returns true if the mode of given network is ModeUserV2.
func (c *Config) Usernet(name string) (bool, error) {
	if nw, ok := c.Networks[name]; ok {
		return nw.Mode == ModeUserV2, nil
	}
	return false, fmt.Errorf("network %q is not defined", name)
}

// DaemonPath returns the daemon path.
func (c *Config) DaemonPath(daemon string) (string, error) {
	switch daemon {
	case SocketVMNet:
		return c.Paths.SocketVMNet, nil
	default:
		return "", fmt.Errorf("unknown daemon type %q", daemon)
	}
}

// IsDaemonInstalled checks whether the daemon is installed.
func (c *Config) IsDaemonInstalled(daemon string) (bool, error) {
	p, err := c.DaemonPath(daemon)
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
func (c *Config) Sock(name string) string {
	return filepath.Join(c.Paths.VarRun, fmt.Sprintf("socket_vmnet.%s", name))
}

func (c *Config) PIDFile(name, daemon string) string {
	return filepath.Join(c.Paths.VarRun, fmt.Sprintf("%s_%s.pid", name, daemon))
}

func (c *Config) LogFile(name, daemon, stream string) string {
	networksDir, _ := dirnames.LimaNetworksDir()
	return filepath.Join(networksDir, fmt.Sprintf("%s_%s.%s.log", name, daemon, stream))
}

func (c *Config) User(daemon string) (osutil.User, error) {
	if ok, _ := c.IsDaemonInstalled(daemon); !ok {
		daemonPath, _ := c.DaemonPath(daemon)
		return osutil.User{}, fmt.Errorf("daemon %q (path=%q) is not available", daemon, daemonPath)
	}
	//nolint:gocritic // singleCaseSwitch: should rewrite switch statement to if statement
	switch daemon {
	case SocketVMNet:
		return osutil.LookupUser("root")
	}
	return osutil.User{}, fmt.Errorf("daemon %q not defined", daemon)
}

func (c *Config) MkdirCmd() string {
	return fmt.Sprintf("/bin/mkdir -m 775 -p %s", c.Paths.VarRun)
}

func (c *Config) StartCmd(name, daemon string) string {
	if ok, _ := c.IsDaemonInstalled(daemon); !ok {
		panic(fmt.Errorf("daemon %q is not available", daemon))
	}
	var cmd string
	switch daemon {
	case SocketVMNet:
		nw := c.Networks[name]
		if c.Paths.SocketVMNet == "" {
			panic("c.Paths.SocketVMNet is empty")
		}
		cmd = fmt.Sprintf("%s --pidfile=%s --socket-group=%s --vmnet-mode=%s",
			c.Paths.SocketVMNet, c.PIDFile(name, SocketVMNet), c.Group, nw.Mode)
		switch nw.Mode {
		case ModeBridged:
			cmd += fmt.Sprintf(" --vmnet-interface=%s", nw.Interface)
		case ModeHost, ModeShared:
			cmd += fmt.Sprintf(" --vmnet-gateway=%s --vmnet-dhcp-end=%s --vmnet-mask=%s",
				nw.Gateway, nw.DHCPEnd, nw.NetMask)
		}
		cmd += " " + c.Sock(name)
	default:
		panic(fmt.Errorf("unexpected daemon %q", daemon))
	}
	return cmd
}

func (c *Config) StopCmd(name, daemon string) string {
	return fmt.Sprintf("/usr/bin/pkill -F %s", c.PIDFile(name, daemon))
}
