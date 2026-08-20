// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/sudoers"
)

// sudoersDaemon is an installed daemon, resolved for rendering its grants.
type sudoersDaemon struct {
	name string
	path string
	user osutil.User
}

func (c Config) Sudoers() (string, error) {
	var daemons []sudoersDaemon
	for _, daemon := range RequiredDaemons() {
		if ok, err := c.IsDaemonInstalled(daemon); err != nil {
			return "", err
		} else if !ok {
			continue
		}
		user, err := c.User(daemon)
		if err != nil {
			return "", err
		}
		path, err := c.DaemonPath(daemon)
		if err != nil {
			return "", err
		}
		daemons = append(daemons, sudoersDaemon{name: daemon, path: path, user: user})
	}
	// On Linux, lima-privileged-net creates the varRun directory itself.
	return c.sudoers(runtime.GOOS == "darwin", daemons)
}

// sudoers renders the grants for already resolved daemons, so that the rendering
// can be tested on a host where they are not installed.
func (c Config) sudoers(mkdir bool, daemons []sudoersDaemon) (string, error) {
	var sb strings.Builder
	if mkdir {
		sb.WriteString(sudoers.NOPASSWD("%"+c.Group, "root", "wheel", c.MkdirCmd()))
	}

	// names must be in stable order to be able to check if sudoers file needs updating
	names := make([]string, 0, len(c.Networks))
	for name, nw := range c.Networks {
		if nw.Mode == ModeUserV2 {
			continue // no sudo needed
		}
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		sb.WriteRune('\n')
		fmt.Fprintf(&sb, "# Manage %q network daemons\n", name)
		for _, d := range daemons {
			cmds := []string{c.startCmd(name, d.name, d.path)}
			if d.name == LimaPrivilegedNet {
				// The wildcard stands for the instance name; the helper derives
				// the tap name itself, from SUDO_UID.
				cmds = append(cmds, c.tapCmdPattern(d.path, name))
				var err error
				if cmds, err = sudoers.Pin(d.path, cmds...); err != nil {
					return "", err
				}
			}
			// Not pinned: a system binary, updated independently of Lima.
			cmds = append(cmds, c.StopCmd(name, d.name))
			sb.WriteRune('\n')
			sb.WriteString(sudoers.NOPASSWD("%"+c.Group, d.user.User, d.user.Group, cmds...))
		}
	}
	return strings.TrimPrefix(sb.String(), "\n"), nil
}

// runAs lists the identities the generated grants are exercised as.
func (c *Config) runAs() ([]sudoers.RunAs, error) {
	var runAs []sudoers.RunAs
	for _, daemon := range RequiredDaemons() {
		if ok, err := c.IsDaemonInstalled(daemon); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		user, err := c.User(daemon)
		if err != nil {
			return nil, err
		}
		runAs = append(runAs, sudoers.RunAs{User: user.User, Group: user.Group})
	}
	return runAs, nil
}

func (c *Config) VerifySudoAccess(ctx context.Context, sudoersFile string) error {
	runAs, err := c.runAs()
	if err != nil {
		return err
	}
	if sudoersFile == "" {
		if err := sudoers.VerifyPasswordLess(ctx, runAs...); err != nil {
			return fmt.Errorf("sudo seems to require a password: %w", err)
		}
		logrus.Debug("sudo doesn't seem to require a password")
		return nil
	}
	fragment, err := c.Sudoers()
	if err != nil {
		return err
	}
	hint := sudoers.RegenerateHint(os.Args[0], sudoersFile, nil)
	return sudoers.VerifyFragment(ctx, sudoersFile, fragment, hint, runAs...)
}
