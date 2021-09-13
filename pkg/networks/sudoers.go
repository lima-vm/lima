package networks

import (
	"fmt"
	"os"
	"sort"
	"strings"
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

func CheckSudoers(sudoersFile string) error {
	b, err := os.ReadFile(sudoersFile)
	if err != nil {
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
