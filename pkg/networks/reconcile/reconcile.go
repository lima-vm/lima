// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package reconcile

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/limatype"
	"github.com/lima-vm/lima/v2/pkg/limatype/dirnames"
	"github.com/lima-vm/lima/v2/pkg/networks"
	"github.com/lima-vm/lima/v2/pkg/networks/usernet"
	"github.com/lima-vm/lima/v2/pkg/osutil"
	"github.com/lima-vm/lima/v2/pkg/store"
)

func Reconcile(ctx context.Context, newInst string) error {
	cfg, err := networks.LoadConfig()
	if err != nil {
		return err
	}
	instances, err := store.Instances()
	if err != nil {
		return err
	}
	activeNetwork := make(map[string]bool, 3)
	for _, instName := range instances {
		instance, err := store.Inspect(ctx, instName)
		if err != nil {
			return err
		}
		// newInst is about to be started, so its networks should be running
		if instance.Status != limatype.StatusRunning && instName != newInst {
			continue
		}
		for _, nw := range instance.Networks {
			if nw.Lima == "" {
				continue
			}
			if _, ok := cfg.Networks[nw.Lima]; !ok {
				logrus.Errorf("network %#q (used by instance %#q) is missing from networks.yaml", nw.Lima, instName)
				continue
			}
			activeNetwork[nw.Lima] = true
		}
	}
	for name := range cfg.Networks {
		var err error
		if activeNetwork[name] {
			err = startNetwork(ctx, &cfg, name)
		} else {
			err = stopNetwork(ctx, &cfg, name)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func sudo(ctx context.Context, user, group, command string) error {
	args := []string{"--user", user, "--group", group, "--non-interactive"}
	args = append(args, strings.Split(command, " ")...)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "sudo", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	logrus.Debugf("Running: %v", cmd.Args)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run %v: stdout=%#q, stderr=%#q: %w",
			cmd.Args, stdout.String(), stderr.String(), err)
	}
	return nil
}

func makeVarRun(ctx context.Context, cfg *networks.Config) error {
	err := sudo(ctx, "root", "wheel", cfg.MkdirCmd())
	if err != nil {
		return err
	}

	// Check that VarRun is daemon-group writable. If we don't report it here, the error would only be visible
	// in the vde_switch daemon log. This has not been checked by networks.Validate() because only the VarRun
	// directory itself needs to be daemon-group writable, any parents just need to be daemon-group executable.
	fi, err := os.Stat(cfg.Paths.VarRun)
	if err != nil {
		return err
	}
	stat, ok := osutil.SysStat(fi)
	if !ok {
		// should never happen
		return fmt.Errorf("could not retrieve stat buffer for %#q", cfg.Paths.VarRun)
	}
	daemon, err := osutil.LookupUser("daemon")
	if err != nil {
		return err
	}
	if fi.Mode()&0o20 == 0 || stat.Gid != daemon.Gid {
		return fmt.Errorf("%#q doesn't seem to be writable by the daemon (gid:%d) group",
			cfg.Paths.VarRun, daemon.Gid)
	}
	return nil
}

// startDaemon starts the daemon process and returns its running *exec.Cmd so the
// caller can wait on the process while polling for its PID file.
func startDaemon(ctx context.Context, cfg *networks.Config, name, daemon string) (*exec.Cmd, error) {
	if err := makeVarRun(ctx, cfg); err != nil {
		return nil, err
	}
	networksDir, err := dirnames.LimaNetworksDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(networksDir, 0o755); err != nil {
		return nil, err
	}
	user, err := cfg.User(daemon)
	if err != nil {
		return nil, err
	}

	args := []string{"--user", user.User, "--group", user.Group, "--non-interactive"}
	args = append(args, strings.Split(cfg.StartCmd(name, daemon), " ")...)
	cmd := exec.CommandContext(ctx, "sudo", args...)
	// set directory to a path the daemon user has read access to because vde_switch calls getcwd() which
	// can fail when called from directories like ~/Downloads, which has 700 permissions
	cmd.Dir = cfg.Paths.VarRun

	stdoutPath := cfg.LogFile(name, daemon, "stdout")
	stderrPath := cfg.LogFile(name, daemon, "stderr")
	if err := os.RemoveAll(stdoutPath); err != nil {
		return nil, err
	}
	if err := os.RemoveAll(stderrPath); err != nil {
		return nil, err
	}

	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, err
	}
	// The child inherits its own dup of the descriptor at Start, so we can close our
	// copy when startDaemon returns; this avoids leaking a descriptor on every retry.
	defer stdoutFile.Close()
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return nil, err
	}
	defer stderrFile.Close()
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	logrus.Debugf("Starting %#q daemon for %#q network: %v", daemon, name, cmd.Args)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to run %v: %w (Hint: check %#q, %#q)", cmd.Args, err, stdoutPath, stderrPath)
	}
	return cmd, nil
}

var validation struct {
	sync.Once
	err error
}

func validateConfig(ctx context.Context, cfg *networks.Config) error {
	validation.Do(func() {
		// make sure all cfg.Paths.* are secure
		validation.err = cfg.Validate()
		if validation.err == nil {
			validation.err = cfg.VerifySudoAccess(ctx, cfg.Paths.Sudoers)
		}
	})
	return validation.err
}

func startNetwork(ctx context.Context, cfg *networks.Config, name string) error {
	logrus.Debugf("Make sure %#q network is running", name)

	// Handle usernet first without sudo requirements
	isUsernet, err := cfg.Usernet(name)
	if err != nil {
		return err
	}
	if isUsernet {
		if err := usernet.Start(ctx, name); err != nil {
			return fmt.Errorf("failed to start usernet %#q: %w", name, err)
		}
		return nil
	}

	if runtime.GOOS != "darwin" {
		return nil
	}

	if err := validateConfig(ctx, cfg); err != nil {
		return err
	}
	var daemons []string
	ok, err := cfg.IsDaemonInstalled(networks.SocketVMNet)
	if err != nil {
		return err
	}
	if ok {
		daemons = append(daemons, networks.SocketVMNet)
	} else {
		return fmt.Errorf("daemon %#q needs to be installed", networks.SocketVMNet)
	}
	for _, daemon := range daemons {
		pid, _ := store.ReadPIDFile(cfg.PIDFile(name, daemon))
		if pid == 0 {
			logrus.Infof("Starting %s daemon for %#q network", daemon, name)
			if err := startDaemonWithRetry(ctx, cfg, name, daemon); err != nil {
				return err
			}
		}
	}
	return nil
}

// daemonStuckError reports that the daemon process is still running but never wrote
// its PID file within the timeout. Unlike an early exit (the transient XPC race we
// retry for), this points at a different problem and is not retried.
type daemonStuckError struct {
	timeout time.Duration
}

func (e *daemonStuckError) Error() string {
	return fmt.Sprintf("did not write its PID file within %s but is still running", e.timeout)
}

// daemonExitedError reports that the daemon process exited before writing its PID
// file. This is the transient VMNET_FAILURE startup race that startDaemonWithRetry
// retries for. err is the process's exit error, which may be nil when the daemon
// exited cleanly (status 0) without writing a PID file.
type daemonExitedError struct {
	err error
}

func (e *daemonExitedError) Error() string {
	if e.err == nil {
		return "daemon exited before writing its PID file"
	}
	return fmt.Sprintf("daemon exited before writing its PID file: %v", e.err)
}

func (e *daemonExitedError) Unwrap() error { return e.err }

// startDaemonWithRetry starts the daemon and waits for it to come up, retrying with
// exponential backoff. socket_vmnet can fail transiently at boot when
// com.apple.NetworkSharing has not yet registered its XPC endpoint (VMNET_FAILURE);
// in that case the process exits before writing its PID file and we retry. Every
// other failure — a process that keeps running without writing its PID file, a
// failure from startDaemon itself, an unreadable PID file, or context cancellation —
// is not transient and is returned immediately.
func startDaemonWithRetry(ctx context.Context, cfg *networks.Config, name, daemon string) error {
	const (
		maxAttempts = 5
		// startTimeout is deliberately generous: socket_vmnet writes its PID file
		// promptly on success, so reaching this while the process is still alive
		// indicates a problem other than the transient startup race.
		startTimeout = 30 * time.Second
	)
	pidFile := cfg.PIDFile(name, daemon)
	stderrLog := cfg.LogFile(name, daemon, "stderr")
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(1<<uint(attempt-2)) * time.Second // 1s, 2s, 4s, 8s
			logrus.Infof("Retrying %s daemon for %#q network (attempt %d/%d, waiting %s): %v",
				daemon, name, attempt, maxAttempts, backoff, lastErr)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
		cmd, err := startDaemon(ctx, cfg, name, daemon)
		if err != nil {
			return err // infrastructure failure (sudo/exec) — not transient
		}
		err = waitForDaemon(ctx, cmd, pidFile, startTimeout)
		if err == nil {
			return nil
		}
		// Only an early exit (the transient XPC race) is retried; everything else
		// is surfaced immediately.
		var exited *daemonExitedError
		if !errors.As(err, &exited) {
			// A stuck (still-running) daemon gets its stderr appended; PID-read and
			// context errors propagate as-is.
			var stuck *daemonStuckError
			if errors.As(err, &stuck) {
				return fmt.Errorf("%s daemon for %#q network %w%s", daemon, name, err, stderrHint(stderrLog))
			}
			return err
		}
		lastErr = err // process exited early — retry
	}
	return fmt.Errorf("%s daemon for %#q network failed to start after %d attempts: %w%s",
		daemon, name, maxAttempts, lastErr, stderrHint(stderrLog))
}

// waitForDaemon waits until the daemon writes its PID file (success), exits without
// one (returns *daemonExitedError so the caller can retry), or stays alive past
// timeout without a PID file (returns *daemonStuckError). A stuck or cancelled
// process is killed and reaped.
func waitForDaemon(ctx context.Context, cmd *exec.Cmd, pidFile string, timeout time.Duration) error {
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		ready, err := pidFileWritten(pidFile)
		if err != nil {
			_ = cmd.Process.Kill()
			<-waitCh
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			<-waitCh
			return ctx.Err()
		case waitErr := <-waitCh:
			// Process exited. If it managed to write the PID file first, treat as success;
			// otherwise this is the transient VMNET_FAILURE case — signal a retry, keeping
			// the exit error so the cause survives even when stderr is empty. waitErr may be
			// nil (clean exit without a PID file); daemonExitedError renders that without a
			// dangling %!w(<nil>).
			ready, err := pidFileWritten(pidFile)
			if err != nil {
				return err
			}
			if ready {
				return nil
			}
			return &daemonExitedError{err: waitErr}
		case <-timer.C:
			_ = cmd.Process.Kill()
			<-waitCh
			return &daemonStuckError{timeout: timeout}
		case <-ticker.C:
			// re-check the PID file on the next loop iteration
		}
	}
}

// pidFileWritten reports whether the daemon has written a valid PID file. A read
// error (e.g. a corrupt or unreadable PID file) is non-transient and is returned so
// the caller can fail fast rather than mistake it for a daemon that is slow to start.
func pidFileWritten(pidFile string) (bool, error) {
	pid, err := store.ReadPIDFile(pidFile)
	if err != nil {
		return false, fmt.Errorf("failed to read PID file %#q: %w", pidFile, err)
	}
	return pid != 0, nil
}

// stderrHint returns the daemon's stderr (or a pointer to the log) to append to errors.
func stderrHint(stderrLog string) string {
	if b, err := os.ReadFile(stderrLog); err == nil && len(b) > 0 {
		return fmt.Sprintf(" (stderr: %s)", strings.TrimSpace(string(b)))
	}
	return fmt.Sprintf(" (check %#q)", stderrLog)
}

func stopNetwork(ctx context.Context, cfg *networks.Config, name string) error {
	logrus.Debugf("Make sure %#q network is stopped", name)
	// Handle usernet first without sudo requirements
	isUsernet, err := cfg.Usernet(name)
	if err != nil {
		return err
	}
	if isUsernet {
		if err := usernet.Stop(ctx, name); err != nil {
			return fmt.Errorf("failed to stop usernet %#q: %w", name, err)
		}
		return nil
	}

	if runtime.GOOS != "darwin" {
		return nil
	}

	// Don't call validateConfig() until we actually need to stop a daemon because
	// stopNetwork() may be called even when the daemons are not installed.
	for _, daemon := range []string{networks.SocketVMNet} {
		if ok, _ := cfg.IsDaemonInstalled(daemon); !ok {
			continue
		}
		pid, _ := store.ReadPIDFile(cfg.PIDFile(name, daemon))
		if pid != 0 {
			logrus.Infof("Stopping %s daemon for %#q network", daemon, name)
			if err := validateConfig(ctx, cfg); err != nil {
				return err
			}
			user, err := cfg.User(daemon)
			if err != nil {
				return err
			}
			err = sudo(ctx, user.User, user.Group, cfg.StopCmd(name, daemon))
			if err != nil {
				return err
			}
		}
		// wait for daemons to terminate (up to 5s) before stopping, otherwise the sockets may not get deleted which
		// will cause subsequent start commands to fail.
		startWaiting := time.Now()
		for {
			if pid, _ := store.ReadPIDFile(cfg.PIDFile(name, daemon)); pid == 0 {
				break
			}
			if time.Since(startWaiting) > 5*time.Second {
				logrus.Infof("%#q daemon for %#q network still running after 5 seconds", daemon, name)
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}
