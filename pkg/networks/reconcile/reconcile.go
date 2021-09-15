package networks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/sirupsen/logrus"
)

func Reconcile(ctx context.Context, newInst string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	config, err := networks.Config()
	if err != nil {
		return err
	}

	instances, err := store.Instances()
	if err != nil {
		return err
	}

	activeNetwork := make(map[string]bool, 3)
	for _, instName := range instances {
		instance, err := store.Inspect(instName)
		if err != nil {
			return err
		}
		// newInst is about to be started, so its networks should be running
		if instance.Status != store.StatusRunning && instName != newInst {
			continue
		}
		for _, nw := range instance.Networks {
			if nw.Lima == "" {
				continue
			}
			if _, ok := config.Networks[nw.Lima]; !ok {
				logrus.Errorf("network %q (used by instance %q) is missing from networks.yaml", nw.Lima, instName)
				continue
			}
			activeNetwork[nw.Lima] = true
		}
	}
	for name := range config.Networks {
		var err error
		if activeNetwork[name] {
			err = startNetwork(&config, ctx, name)
		} else {
			err = stopNetwork(&config, name)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func sudo(user, group, command string) error {
	args := []string{"--user", user, "--group", group, "--non-interactive"}
	args = append(args, strings.Split(command, " ")...)
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("sudo", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	logrus.Debugf("Running: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: stdout=%q, stderr=%q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	return nil
}

func startDaemon(config *networks.NetworksConfig, ctx context.Context, name, daemon string) error {
	err := sudo("root", "wheel", config.MkdirCmd())
	if err != nil {
		return err
	}

	networksDir, _ := dirnames.LimaNetworksDir()
	if err := os.MkdirAll(networksDir, 0755); err != nil {
		return err
	}

	stdoutPath := config.LogFile(name, daemon, "stdout")
	stderrPath := config.LogFile(name, daemon, "stderr")
	if err := os.RemoveAll(stdoutPath); err != nil {
		return err
	}
	if err := os.RemoveAll(stderrPath); err != nil {
		return err
	}
	stdoutW, err := os.Create(stdoutPath)
	if err != nil {
		return err
	}
	// no defer stdoutW.Close()
	stderrW, err := os.Create(stderrPath)
	if err != nil {
		return err
	}
	// no defer stderrW.Close()

	args := []string{"--user", config.DaemonUser(daemon), "--group", config.DaemonGroup(daemon), "--non-interactive"}
	args = append(args, strings.Split(config.StartCmd(name, daemon), " ")...)
	cmd := exec.CommandContext(ctx, "sudo", args...)
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW
	logrus.Debugf("Starting %q daemon for %q network: %v", daemon, name, cmd.Args)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

var validation struct {
	sync.Once
	err error
}

func validateConfig(config *networks.NetworksConfig) error {
	validation.Do(func() {
		// make sure all config.Paths.* are secure
		validation.err = config.Validate()
		if validation.err == nil {
			validation.err = config.VerifySudoAccess(config.Paths.Sudoers)
		}
	})
	return validation.err
}

func startNetwork(config *networks.NetworksConfig, ctx context.Context, name string) error {
	logrus.Debugf("Make sure %q network is running", name)
	if err := validateConfig(config); err != nil {
		return err
	}
	for _, daemon := range []string{networks.Switch, networks.VMNet} {
		pid, _ := store.ReadPIDFile(config.PIDFile(name, daemon))
		if pid == 0 {
			logrus.Infof("Starting %s daemon for %q network", daemon, name)
			if err := startDaemon(config, ctx, name, daemon); err != nil {
				return err
			}
		}
	}
	return nil
}

func stopNetwork(config *networks.NetworksConfig, name string) error {
	logrus.Debugf("Make sure %q network is stopped", name)
	// Don't call validateConfig() until we actually need to stop a daemon because
	// stopNetwork() may be called even when the vde daemons are not installed.
	for _, daemon := range []string{networks.VMNet, networks.Switch} {
		pid, _ := store.ReadPIDFile(config.PIDFile(name, daemon))
		if pid != 0 {
			logrus.Infof("Stopping %s daemon for %q network", daemon, name)
			if err := validateConfig(config); err != nil {
				return err
			}
			err := sudo(config.DaemonUser(daemon), config.DaemonGroup(daemon), config.StopCmd(name, daemon))
			if err != nil {
				return err
			}
		}
		// TODO: wait for VMNet to terminate before stopping Switch, otherwise the socket may not get deleted
	}
	return nil
}
