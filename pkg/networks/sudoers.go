package networks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

func Sudoers() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%%%s ALL=(root:wheel) NOPASSWD:NOSETENV: %s\n", cfg.Group, cfg.MkdirCmd()))

	// names must be in stable order to be able to check if sudoers file needs updating
	names := make([]string, 0, len(cfg.Networks))
	for name, nw := range cfg.Networks {
		if nw.Mode == ModeUserV2 {
			continue // no sudo needed
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sb.WriteRune('\n')
		sb.WriteString(fmt.Sprintf("# Manage %q network daemons\n", name))
		for _, daemon := range []string{SocketVMNet} {
			if ok, err := cfg.IsDaemonInstalled(daemon); err != nil {
				return "", err
			} else if !ok {
				continue
			}
			user, err := cfg.User(daemon)
			if err != nil {
				return "", err
			}
			sb.WriteRune('\n')
			sb.WriteString(fmt.Sprintf("%%%s ALL=(%s:%s) NOPASSWD:NOSETENV: \\\n",
				cfg.Group, user.User, user.Group))
			sb.WriteString(fmt.Sprintf("    %s, \\\n", cfg.StartCmd(name, daemon)))
			sb.WriteString(fmt.Sprintf("    %s\n", cfg.StopCmd(name, daemon)))
		}
	}
	return sb.String(), nil
}

func (c *Config) passwordLessSudo() error {
	// Flush cached sudo password
	cmd := exec.Command("sudo", "-k")
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
		cmd = exec.Command("sudo", "--user", user.User, "--group", user.Group, "--non-interactive", "true")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
		}
	}
	return nil
}

func (c *Config) VerifySudoAccess(sudoersFile string) error {
	if sudoersFile == "" {
		err := c.passwordLessSudo()
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
			err = c.passwordLessSudo()
			if err == nil {
				logrus.Debugf("%q does not exist, but sudo doesn't seem to require a password", sudoersFile)
				return nil
			}
			logrus.Debugf("%q does not exist; passwordLessSudo error: %s", sudoersFile, err)
		}
		return fmt.Errorf("can't read %q: %w: (Hint: %s)", sudoersFile, err, hint)
	}
	sudoers, err := Sudoers()
	if err != nil {
		return err
	}
	if string(b) != sudoers {
		// Happens on upgrading socket_vmnet with Homebrew
		return fmt.Errorf("sudoers file %q is out of sync and must be regenerated (Hint: %s)", sudoersFile, hint)
	}
	return nil
}
