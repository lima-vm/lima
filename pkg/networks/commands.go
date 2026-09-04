// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/opencontainers/go-digest"

	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

const (
	SocketVMNet       = "socket_vmnet"
	LimaPrivilegedNet = "lima-privileged-net"

	// maxIfNameLen is the longest name the kernel accepts for a network
	// interface (IFNAMSIZ-1). Both the bridge and the tap names below have to fit
	// into it; Validate() rejects network names that would overflow the bridge.
	maxIfNameLen = 15
	// bridgePrefix is prepended to the network name to name the bridge that
	// lima-privileged-net creates for "shared" and "host" networks.
	bridgePrefix = "lima-"
	// tapPrefix is prepended to a hash of the instance and network name.
	tapPrefix = "limatap"
	// tapDigits is the number of hex digits following tapPrefix.
	// len(tapPrefix)+tapDigits must not exceed maxIfNameLen.
	tapDigits = 8

	// privilegedSubdir is the libexec/lima subdirectory reserved for the helpers
	// that run with elevated privileges.
	privilegedSubdir = "privileged"
)

// RequiredDaemons returns the privileged helpers needed by the non-usernet
// networks on this host: socket_vmnet on macOS, lima-privileged-net on Linux.
// Everything else in this package is generic over the daemon name.
func RequiredDaemons() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{SocketVMNet}
	case "linux":
		return []string{LimaPrivilegedNet}
	default:
		return nil
	}
}

// Commands in `sudoers` cannot use quotes, so all arguments are printed via "%s"
// and not "%q". cfg.Paths.* entries must not include any whitespace!

func (c *Config) Check(name string) error {
	if _, ok := c.Networks[name]; ok {
		return nil
	}
	return fmt.Errorf("network %#q is not defined", name)
}

// Usernet returns true if the mode of given network is ModeUserV2.
func (c *Config) Usernet(name string) (bool, error) {
	if nw, ok := c.Networks[name]; ok {
		return nw.Mode == ModeUserV2, nil
	}
	return false, fmt.Errorf("network %#q is not defined", name)
}

// DaemonPath returns the daemon path.
func (c *Config) DaemonPath(daemon string) (string, error) {
	switch daemon {
	case SocketVMNet:
		return c.Paths.SocketVMNet, nil
	case LimaPrivilegedNet:
		return limaPrivilegedNetPath()
	default:
		return "", fmt.Errorf("unknown daemon type %#q", daemon)
	}
}

// DigestSpec returns the sudoers `Digest_Spec` pinning the contents of the daemon
// binary. sudo (>= 1.8.7) hashes the file immediately before executing it and
// refuses to run it when the digest no longer matches, so an attacker who manages
// to replace the helper cannot get the replacement executed as root. Unlike the
// ownership checks in validate.go, this is enforced inside sudo itself and is
// therefore not subject to a time-of-check/time-of-use race with limactl.
func (c *Config) DigestSpec(daemon string) (string, error) {
	path, err := c.DaemonPath(daemon)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	d, err := digest.FromReader(f)
	if err != nil {
		return "", err
	}
	return d.String(), nil
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
		return osutil.User{}, fmt.Errorf("daemon %#q (path=%#q) is not available", daemon, daemonPath)
	}
	switch daemon {
	case SocketVMNet, LimaPrivilegedNet:
		return osutil.LookupUser("root")
	}
	return osutil.User{}, fmt.Errorf("daemon %#q not defined", daemon)
}

func (c *Config) MkdirCmd() string {
	return fmt.Sprintf("/bin/mkdir -m 775 -p %s", c.Paths.VarRun)
}

func (c *Config) StartCmd(name, daemon string) string {
	if ok, _ := c.IsDaemonInstalled(daemon); !ok {
		panic(fmt.Errorf("daemon %#q is not available", daemon))
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
	case LimaPrivilegedNet:
		nw := c.Networks[name]
		privilegedNetPath, err := limaPrivilegedNetPath()
		if err != nil {
			panic(fmt.Errorf("failed to get lima-privileged-net path: %w", err))
		}
		cmd = fmt.Sprintf("%s start --pidfile=%s --mode=%s --bridge=%s",
			privilegedNetPath, c.PIDFile(name, LimaPrivilegedNet), nw.Mode, c.BridgeName(name))
		if nw.Mode != ModeBridged {
			cmd += fmt.Sprintf(" --gateway=%s --dhcp-end=%s --netmask=%s",
				nw.Gateway, nw.DHCPEnd, nw.NetMask)
		}
	default:
		panic(fmt.Errorf("unexpected daemon %#q", daemon))
	}
	return cmd
}

func (c *Config) StopCmd(name, daemon string) string {
	return fmt.Sprintf("/usr/bin/pkill -F %s", c.PIDFile(name, daemon))
}

// IsManagedBridge reports whether the interface is a bridge created by
// lima-privileged-net.
func IsManagedBridge(name string) bool {
	return strings.HasPrefix(name, bridgePrefix)
}

var tapNameRegex = regexp.MustCompile(fmt.Sprintf(`^%s[0-9a-f]{%d}$`, regexp.QuoteMeta(tapPrefix), tapDigits))

// IsTapName reports whether the interface name could have been generated by
// TapName, i.e. whether the interface belongs to Lima.
func IsTapName(name string) bool {
	return tapNameRegex.MatchString(name)
}

// BridgeName returns the bridge that the instances of a network are attached to:
// the pre-existing bridge named by `interface` for "bridged" networks, and the
// Lima-managed "lima-<name>" bridge otherwise.
func (c *Config) BridgeName(name string) string {
	if nw := c.Networks[name]; nw.Mode == ModeBridged {
		return nw.Interface
	}
	return bridgePrefix + name
}

// TapCmd returns the command creating the tap device that connects an instance
// to the bridge of a network. Passing TapNamePattern renders the sudoers entry.
func (c *Config) TapCmd(name, tap string) string {
	privilegedNetPath, err := limaPrivilegedNetPath()
	if err != nil {
		panic(fmt.Errorf("failed to get lima-privileged-net path: %w", err))
	}
	return fmt.Sprintf("%s tap --bridge=%s %s", privilegedNetPath, c.BridgeName(name), tap)
}

// TapName returns the name of the tap device connecting an instance to a
// network. A hash keeps the name within the maxIfNameLen limit and within the
// fixed shape that TapNamePattern pins down in the sudoers file. The uid is part
// of the hash because the bridges are shared between all users of the sudoers
// group, and a tap device may only ever be used by its owner.
func TapName(instName, netName string) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d/%s/%s", os.Getuid(), instName, netName))
	return fmt.Sprintf("%s%x", tapPrefix, sum)[:len(tapPrefix)+tapDigits]
}

// TapNamePattern is the sudoers wildcard matching exactly the names generated by
// TapName. Character classes are used instead of "*" because "*" would also
// match whitespace, which would allow smuggling extra arguments into the command.
func TapNamePattern() string {
	return tapPrefix + strings.Repeat("[0-9a-f]", tapDigits)
}
