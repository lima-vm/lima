// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package networks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/sudoers"
)

func (c Config) Sudoers() (string, error) {
	var sb strings.Builder
	sb.WriteString(sudoers.NOPASSWD("%"+c.Group, "root", "wheel", c.MkdirCmd()))

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
		for _, daemon := range []string{SocketVMNet} {
			if ok, err := c.IsDaemonInstalled(daemon); err != nil {
				return "", err
			} else if !ok {
				continue
			}
			user, err := c.User(daemon)
			if err != nil {
				return "", err
			}
			sb.WriteRune('\n')
			sb.WriteString(sudoers.NOPASSWD("%"+c.Group, user.User, user.Group, c.StartCmd(name, daemon), c.StopCmd(name, daemon)))
		}
	}
	return sb.String(), nil
}

func (c *Config) passwordLessSudo(ctx context.Context) error {
	// Flush cached sudo password
	cmd := exec.CommandContext(ctx, "sudo", "-k")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}
	// Verify that user/groups for both daemons work without a password, e.g.
	// %admin ALL = (ALL:ALL) NOPASSWD: ALL
	for _, daemon := range []string{SocketVMNet} {
		if ok, err := c.IsDaemonInstalled(daemon); err != nil {
			return err
		} else if !ok {
			continue
		}
		user, err := c.User(daemon)
		if err != nil {
			return err
		}
		if err := sudoers.Run(ctx, user.User, user.Group, nil, nil, nil, "", "true"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) VerifySudoAccess(ctx context.Context, sudoersFile string) error {
	if sudoersFile == "" {
		err := c.passwordLessSudo(ctx)
		if err == nil {
			logrus.Debug("sudo doesn't seem to require a password")
			return nil
		}
		return fmt.Errorf("passwordLessSudo error: %w", err)
	}
	hint := fmt.Sprintf("run `%s sudoers >etc_sudoers.d_lima && sudo install -o root etc_sudoers.d_lima %q`)",
		os.Args[0], sudoersFile)
	b, err := os.ReadFile(sudoersFile)
	if err != nil {
		// Default networks.yaml specifies /etc/sudoers.d/lima file. Don't throw an error when the
		// file doesn't exist, as long as password-less sudo still works.
		if errors.Is(err, os.ErrNotExist) {
			err = c.passwordLessSudo(ctx)
			if err == nil {
				logrus.Debugf("%q does not exist, but sudo doesn't seem to require a password", sudoersFile)
				return nil
			}
			logrus.Debugf("%q does not exist; passwordLessSudo error: %s", sudoersFile, err)
		}
		return fmt.Errorf("can't read %q: %w: (Hint: %s)", sudoersFile, err, hint)
	}
	sudoers, err := c.Sudoers()
	if err != nil {
		return err
	}
	// limactl sudoers writes a single file that may include additional
	// non-network entries. Network startup only needs its own generated
	// fragment to be present verbatim.
	if !strings.Contains(string(b), sudoers) {
		// Happens on upgrading socket_vmnet with Homebrew
		return fmt.Errorf("sudoers file %q is out of sync and must be regenerated (Hint: %s)", sudoersFile, hint)
	}
	return nil
}
