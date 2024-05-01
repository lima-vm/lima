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
	"time"

	"github.com/lima-vm/lima/pkg/networks"
	"github.com/lima-vm/lima/pkg/networks/usernet"
	"github.com/lima-vm/lima/pkg/osutil"
	"github.com/lima-vm/lima/pkg/store"
	"github.com/lima-vm/lima/pkg/store/dirnames"
	"github.com/sirupsen/logrus"
)

func Reconcile(ctx context.Context, newInst string) error {
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
			err = startNetwork(ctx, &config, name)
		} else {
			err = stopNetwork(ctx, &config, name)
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

func makeVarRun(config *networks.YAML) error {
	err := sudo("root", "wheel", config.MkdirCmd())
	if err != nil {
		return err
	}

	// Check that VarRun is daemon-group writable. If we don't report it here, the error would only be visible
	// in the vde_switch daemon log. This has not been checked by networks.Validate() because only the VarRun
	// directory itself needs to be daemon-group writable, any parents just need to be daemon-group executable.
	fi, err := os.Stat(config.Paths.VarRun)
	if err != nil {
		return err
	}
	stat, ok := osutil.SysStat(fi)
	if !ok {
		// should never happen
		return fmt.Errorf("could not retrieve stat buffer for %q", config.Paths.VarRun)
	}
	daemon, err := osutil.LookupUser("daemon")
	if err != nil {
		return err
	}
	if fi.Mode()&0o20 == 0 || stat.Gid != daemon.Gid {
		return fmt.Errorf("%q doesn't seem to be writable by the daemon (gid:%d) group",
			config.Paths.VarRun, daemon.Gid)
	}
	return nil
}

func startDaemon(ctx context.Context, config *networks.YAML, name, daemon string) error {
	if err := makeVarRun(config); err != nil {
		return err
	}
	networksDir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(networksDir, 0o755); err != nil {
		return err
	}
	user, err := config.User(daemon)
	if err != nil {
		return err
	}

	args := []string{"--user", user.User, "--group", user.Group, "--non-interactive"}
	args = append(args, strings.Split(config.StartCmd(name, daemon), " ")...)
	cmd := exec.CommandContext(ctx, "sudo", args...)
	// set directory to a path the daemon user has read access to because vde_switch calls getcwd() which
	// can fail when called from directories like ~/Downloads, which has 700 permissions
	cmd.Dir = config.Paths.VarRun

	stdoutPath := config.LogFile(name, daemon, "stdout")
	stderrPath := config.LogFile(name, daemon, "stderr")
	if err := os.RemoveAll(stdoutPath); err != nil {
		return err
	}
	if err := os.RemoveAll(stderrPath); err != nil {
		return err
	}

	cmd.Stdout, err = os.Create(stdoutPath)
	if err != nil {
		return err
	}
	cmd.Stderr, err = os.Create(stderrPath)
	if err != nil {
		return err
	}

	logrus.Debugf("Starting %q daemon for %q network: %v", daemon, name, cmd.Args)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run %v: %w (Hint: check %q, %q)", cmd.Args, err, stdoutPath, stderrPath)
	}
	return nil
}

var validation struct {
	sync.Once
	err error
}

func validateConfig(config *networks.YAML) error {
	validation.Do(func() {
		// make sure all config.Paths.* are secure
		validation.err = config.Validate()
		if validation.err == nil {
			validation.err = config.VerifySudoAccess(config.Paths.Sudoers)
		}
	})
	return validation.err
}

func startNetwork(ctx context.Context, config *networks.YAML, name string) error {
	logrus.Debugf("Make sure %q network is running", name)

	// Handle usernet first without sudo requirements
	isUsernet, err := config.Usernet(name)
	if err != nil {
		return err
	}
	if isUsernet {
		if err := usernet.Start(ctx, name); err != nil {
			return fmt.Errorf("failed to start usernet %q: %w", name, err)
		}
		return nil
	}

	if runtime.GOOS != "darwin" {
		return nil
	}

	if err := validateConfig(config); err != nil {
		return err
	}
	var daemons []string
	ok, err := config.IsDaemonInstalled(networks.SocketVMNet)
	if err != nil {
		return err
	}
	if ok {
		daemons = append(daemons, networks.SocketVMNet)
	} else {
		return fmt.Errorf("daemon %q needs to be installed", networks.SocketVMNet)
	}
	for _, daemon := range daemons {
		pid, _ := store.ReadPIDFile(config.PIDFile(name, daemon))
		if pid == 0 {
			logrus.Infof("Starting %s daemon for %q network", daemon, name)
			if err := startDaemon(ctx, config, name, daemon); err != nil {
				return err
			}
		}
	}
	return nil
}

func stopNetwork(ctx context.Context, config *networks.YAML, name string) error {
	logrus.Debugf("Make sure %q network is stopped", name)
	// Handle usernet first without sudo requirements
	isUsernet, err := config.Usernet(name)
	if err != nil {
		return err
	}
	if isUsernet {
		if err := usernet.Stop(ctx, name); err != nil {
			return fmt.Errorf("failed to stop usernet %q: %w", name, err)
		}
		return nil
	}

	if runtime.GOOS != "darwin" {
		return nil
	}

	// Don't call validateConfig() until we actually need to stop a daemon because
	// stopNetwork() may be called even when the daemons are not installed.
	for _, daemon := range []string{networks.SocketVMNet} {
		if ok, _ := config.IsDaemonInstalled(daemon); !ok {
			continue
		}
		pid, _ := store.ReadPIDFile(config.PIDFile(name, daemon))
		if pid != 0 {
			logrus.Infof("Stopping %s daemon for %q network", daemon, name)
			if err := validateConfig(config); err != nil {
				return err
			}
			user, err := config.User(daemon)
			if err != nil {
				return err
			}
			err = sudo(user.User, user.Group, config.StopCmd(name, daemon))
			if err != nil {
				return err
			}
		}
		// wait for daemons to terminate (up to 5s) before stopping, otherwise the sockets may not get deleted which
		// will cause subsequent start commands to fail.
		startWaiting := time.Now()
		for {
			if pid, _ := store.ReadPIDFile(config.PIDFile(name, daemon)); pid == 0 {
				break
			}
			if time.Since(startWaiting) > 5*time.Second {
				logrus.Infof("%q daemon for %q network still running after 5 seconds", daemon, name)
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}
