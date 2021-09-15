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
	config, err := Config()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%%%s ALL=(root:wheel) NOPASSWD:NOSETENV: %s\n", config.Group, config.MkdirCmd()))

	// names must be in stable order to be able to check if sudoers file needs updating
	names := make([]string, 0, len(config.Networks))
	for name := range config.Networks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		sb.WriteRune('\n')
		sb.WriteString(fmt.Sprintf("# Manage %q network daemons\n", name))
		for _, daemon := range []string{Switch, VMNet} {
			prefix := strings.ToUpper(name + "_" + daemon)
			sb.WriteRune('\n')
			sb.WriteString(fmt.Sprintf("Cmnd_Alias %s_START = %s\n", prefix, config.StartCmd(name, daemon)))
			sb.WriteString(fmt.Sprintf("Cmnd_Alias %s_STOP = %s\n", prefix, config.StopCmd(name, daemon)))
			sb.WriteString(fmt.Sprintf("%%%s ALL=(%s:%s) NOPASSWD:NOSETENV: %s_START, %s_STOP\n",
				config.Group, config.DaemonUser(daemon), config.DaemonGroup(daemon), prefix, prefix))
		}
	}
	return sb.String(), nil
}

func (config *NetworksConfig) passwordLessSudo() error {
	// Flush cached sudo password
	cmd := exec.Command("sudo", "-k")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}
	// Verify that user/groups for both daemons work without a password, e.g.
	// %admin ALL = (ALL:ALL) NOPASSWD: ALL
	for _, daemon := range []string{Switch, VMNet} {
		cmd = exec.Command("sudo", "--user", config.DaemonUser(daemon),
			"--group", config.DaemonGroup(daemon), "--non-interactive", "true")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to run %v: %w", cmd.Args, err)
		}
	}
	return nil
}

func (config *NetworksConfig) VerifySudoAccess(sudoersFile string) error {
	if sudoersFile == "" {
		if err := config.passwordLessSudo(); err == nil {
			logrus.Debug("sudo doesn't seem to require a password")
			return nil
		} else {
			return fmt.Errorf("passwordLessSudo error: %w", err)
		}
	}
	b, err := os.ReadFile(sudoersFile)
	if err != nil {
		// Default networks.yaml specifies /etc/sudoers.d/lima file. Don't throw an error when the
		// file doesn't exist, as long as password-less sudo still works.
		if errors.Is(err, os.ErrNotExist) {
			if err := config.passwordLessSudo(); err == nil {
				logrus.Debugf("%q does not exist, but sudo doesn't seem to require a password", sudoersFile)
				return nil
			} else {
				logrus.Debugf("%q does not exist; passwordLessSudo error: %s", sudoersFile, err)
			}
		}
		return fmt.Errorf("can't read %q: %s", sudoersFile, err)
	}
	sudoers, err := Sudoers()
	if err != nil {
		return err
	}
	if string(b) != sudoers {
		return fmt.Errorf("sudoers file %q is out of sync and must be regenerated", sudoersFile)
	}
	return nil
}
