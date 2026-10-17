// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/sshutil"
)

const (
	sshMasterMinBackoff   = 1 * time.Second
	sshMasterMaxBackoff   = 30 * time.Second
	sshMasterStableAfter  = 60 * time.Second
	sshMasterPollInterval = 500 * time.Millisecond
	sshMasterExitTimeout  = 10 * time.Second
)

// sshMaster runs the instance's SSH ControlMaster as a child of the host agent
// and restarts it when it exits, so the host agent logs a master death and can
// restore the forwards that the master held.
// It is used on macOS and Linux hosts only, where the clients built on
// sshutil.SSHOpts run with ControlMaster=no and multiplex over this master.
type sshMaster struct {
	sshConfig *ssh.SSHConfig // client options from sshutil.SSHOpts
	instDir   string
	address   string
	port      int

	// replay is called with the new generation each time a master is ready,
	// including the first, so it also makes the forwards recorded while no
	// master was ready yet. It skips forwards already made on that generation.
	replay func(ctx context.Context, gen uint64)

	mu      sync.Mutex
	started bool
	stopped bool
	stopCh  chan struct{} // closed by stopRestarting
	done    chan struct{} // closed when the restart loop returns
	cmd     *exec.Cmd     // the current master process
	gen     uint64        // bumped each time a new master is ready; 0 until the first one is
	ready   bool          // whether cmd is the master currently serving the control socket
}

func newSSHMaster(sshConfig *ssh.SSHConfig, instDir, address string, port int) *sshMaster {
	return &sshMaster{
		sshConfig: sshConfig,
		instDir:   instDir,
		address:   address,
		port:      port,
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// sshMasterArgs returns the arguments of the supervised master. ssh keeps the
// first value it sees for an option, so the master's own control options come
// before the inherited ones; an inherited ControlPersist=yes would otherwise make
// the master background itself out of the supervisor's reach. The inherited
// ControlMaster and ControlPersist are removed as well, so that each option
// appears once. The inherited ControlPath is kept.
// ServerAlive turns a hung connection into an exit within about 60 s.
func sshMasterArgs(clientArgs []string, address string, port int) []string {
	args := []string{
		"-o", "ControlMaster=yes",
		"-o", "ControlPersist=no",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=4",
	}
	args = append(args, sshutil.RemoveOptsFromSSHArgs(clientArgs, "ControlMaster", "ControlPersist")...)
	args = append(args, "-N")
	return append(args, hostArgs(address, port)...)
}

func hostArgs(address string, port int) []string {
	if port == 0 {
		return []string{address}
	}
	return []string{"-p", strconv.Itoa(port), address}
}

// start launches the restart loop on its first call, and returns nil once a
// master started by the loop is serving the control socket. It is a no-op after
// stopRestarting, since the host agent can close while requirements still run.
func (m *sshMaster) start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return nil
	}
	if !m.started {
		m.started = true
		go m.run(ctx)
	}
	if !m.ready {
		return errors.New("the SSH ControlMaster is not ready yet")
	}
	return nil
}

func (m *sshMaster) run(ctx context.Context) {
	defer close(m.done)
	backoff := sshMasterMinBackoff
	for {
		if m.isStopped(ctx) {
			return
		}
		startedAt := time.Now()
		err := m.runOnce(ctx)
		if m.isStopped(ctx) {
			// The host agent ends the master when it closes, so return before
			// the warning.
			return
		}
		logrus.WithError(err).Warn("SSH ControlMaster exited")
		if time.Since(startedAt) >= sshMasterStableAfter {
			backoff = sshMasterMinBackoff
		}
		logrus.Infof("Restarting the SSH ControlMaster in %v", backoff)
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, sshMasterMaxBackoff)
	}
}

func (m *sshMaster) isStopped(ctx context.Context) bool {
	return m.hasStopped() || ctx.Err() != nil
}

// runOnce starts one master, waits until it serves the control socket, and
// returns when it exits.
func (m *sshMaster) runOnce(ctx context.Context) error {
	m.exitForeignMaster(ctx)
	if _, err := sshutil.RemoveStaleControlMaster(ctx, m.instDir); err != nil {
		return fmt.Errorf("failed to remove the stale control socket: %w", err)
	}
	cmd := sshCommand(ctx, m.sshConfig.Binary(), sshMasterArgs(m.sshConfig.Args(), m.address, m.port)...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	logrus.Debugf("starting the SSH ControlMaster: %s %v", cmd.Path, cmd.Args)
	if err := cmd.Start(); err != nil {
		return err
	}
	m.mu.Lock()
	m.cmd = cmd
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.cmd = nil
		m.ready = false
		m.mu.Unlock()
	}()

	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logrus.Infof("[SSH ControlMaster] %s", scanner.Text())
		}
	}()
	exited := make(chan error, 1)
	go func() {
		// Wait must not run before the stderr pipe has been read to EOF.
		<-stderrDone
		exited <- cmd.Wait()
	}()

	ticker := time.NewTicker(sshMasterPollInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-exited:
			if err == nil {
				return errors.New("exited with status 0")
			}
			return err
		case <-ticker.C:
			if m.isReady() {
				continue
			}
			pid, err := m.masterPID(ctx)
			if err != nil {
				continue
			}
			if pid != cmd.Process.Pid {
				// Our ssh found the socket taken and runs as a plain connection.
				logrus.Warnf("The SSH control socket is held by a master (pid %d) that the host agent did not start; replacing it", pid)
				_ = cmd.Process.Kill()
				continue
			}
			m.markReady(ctx)
		}
	}
}

func (m *sshMaster) isReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ready
}

func (m *sshMaster) markReady(ctx context.Context) {
	m.mu.Lock()
	m.ready = true
	m.gen++
	gen := m.gen
	replay := m.replay
	m.mu.Unlock()
	logrus.Infof("SSH ControlMaster is ready (generation %d)", gen)
	if replay != nil {
		replay(ctx, gen)
	}
}

// exitForeignMaster asks a master that the host agent did not start to exit,
// e.g. one left by an earlier host agent or created by a client that still uses
// ControlMaster=auto.
func (m *sshMaster) exitForeignMaster(ctx context.Context) {
	if !sshutil.IsControlMasterRunning(ctx, m.instDir) {
		return
	}
	logrus.Warn("The SSH control socket is held by a master that the host agent did not start; asking it to exit")
	if _, err := m.control(ctx, "exit"); err != nil {
		logrus.WithError(err).Warn("Failed to exit the foreign SSH ControlMaster")
	}
}

var sshMasterPIDRegexp = regexp.MustCompile(`pid=(\d+)`)

// masterPID returns the pid of the master serving the control socket, as
// reported by `ssh -O check`.
func (m *sshMaster) masterPID(ctx context.Context) (int, error) {
	out, err := m.control(ctx, "check")
	if err != nil {
		return 0, err
	}
	match := sshMasterPIDRegexp.FindStringSubmatch(out)
	if match == nil {
		return 0, fmt.Errorf("no pid in %q", out)
	}
	return strconv.Atoi(match[1])
}

// control runs `ssh -O <op>` against the control socket.
func (m *sshMaster) control(ctx context.Context, op string) (string, error) {
	args := slices.Concat(m.sshConfig.Args(), []string{"-O", op}, hostArgs(m.address, m.port))
	cmd := sshCommand(ctx, m.sshConfig.Binary(), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("failed to execute `ssh -O %s`, out=%q: %w", op, string(out), err)
	}
	return string(out), nil
}

// generation returns the generation of the newest ready master. It returns 0
// before the first master is ready, and on Windows hosts, where m is nil.
func (m *sshMaster) generation() uint64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.gen
}

// hasStopped reports whether stopRestarting has run. It reports true on
// Windows hosts, where m is nil, because the host agent does not supervise a
// master there.
func (m *sshMaster) hasStopped() bool {
	if m == nil {
		return true
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

// stopRestarting keeps the restart loop from starting another master. The
// current master keeps running, so the cleanups that cancel forwards over it
// still work.
func (m *sshMaster) stopRestarting() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.stopped {
		m.stopped = true
		close(m.stopCh)
	}
}

// terminate stops restarting and ends the current master. It runs
// `ssh -O exit` first, and kills the master if it is still running after
// sshMasterExitTimeout.
func (m *sshMaster) terminate() error {
	m.stopRestarting()
	m.mu.Lock()
	started := m.started
	cmd := m.cmd
	m.mu.Unlock()
	if !started {
		return nil
	}
	if cmd != nil {
		logrus.Debugf("shutting down the SSH master")
		if _, err := m.control(context.Background(), "exit"); err != nil {
			logrus.WithError(err).Warn("failed to exit SSH master")
		}
	}
	select {
	case <-m.done:
	case <-time.After(sshMasterExitTimeout):
		logrus.Warn("SSH master did not exit; killing it")
		m.mu.Lock()
		if m.cmd != nil {
			_ = m.cmd.Process.Kill()
		}
		m.mu.Unlock()
		<-m.done
	}
	return nil
}
