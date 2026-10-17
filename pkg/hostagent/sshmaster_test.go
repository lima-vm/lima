// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"

	"github.com/lima-vm/lima/v2/pkg/limatype/filenames"
)

func TestSSHMasterArgs(t *testing.T) {
	clientArgs := []string{
		"-F", "/dev/null",
		"-o", "IdentityFile=/key",
		"-o", "ControlMaster=no",
		"-o", `ControlPath="/lima/default/ssh.sock"`,
		"-o", "ControlPersist=yes",
		"-o", "User=me",
	}
	want := []string{
		"-o", "ControlMaster=yes",
		"-o", "ControlPersist=no",
		"-F", "/dev/null",
		"-o", "IdentityFile=/key",
		"-o", `ControlPath="/lima/default/ssh.sock"`,
		"-o", "User=me",
		"-N", "-p", "60022", "127.0.0.1",
	}
	assert.DeepEqual(t, sshMasterArgs(clientArgs, "127.0.0.1", 60022), want)
}

func TestSSHMasterArgsWithoutPort(t *testing.T) {
	assert.DeepEqual(t, sshMasterArgs(nil, "127.0.0.1", 0)[4:], []string{"-N", "127.0.0.1"})
}

// localSSHAddress is the SSH address that the fake-ssh tests give newSSHMaster.
func localSSHAddress() (sshAddress string, sshPort int) { return "127.0.0.1", 0 }

// fakeSSHOnPath puts a fake ssh first on PATH and returns the instance
// directory that it keeps its state in. With -N it runs as the master and
// records its pid, unless foreign.pid names another master; then it runs as a
// plain connection, as ssh does when the control socket is taken. -O check
// reports the master that holds the socket, and -O exit ends it. With -N it
// fails at once while a file named fail exists, appending its pid to failed.pid,
// and as a plain connection it records its pid in plain.pid. The helper runs the
// fake once, so that a slow first exec happens before a test times anything.
func fakeSSHOnPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the host agent does not supervise the master on Windows hosts")
	}
	//nolint:usetesting // t.TempDir() returns a path too long for a unix socket.
	dir, err := os.MkdirTemp("", "lima-ssh")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	script := `#!/bin/sh
cd "$FAKE_SSH_DIR" || exit 255
holder=master.pid
[ -s foreign.pid ] && holder=foreign.pid
case " $* " in
*" -O check "*) [ -s $holder ] || exit 255; echo "Master running (pid=$(cat $holder))" ;;
*" -O exit "*) kill "$(cat $holder)"; rm -f $holder ;;
*" -N "*)
	[ -e fail ] && { echo $$ >>failed.pid; exit 255; }
	if [ -s foreign.pid ]; then echo $$ >plain.pid; else echo $$ >master.pid; fi
	exec sleep 600 ;;
esac
`
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755))
	t.Setenv("FAKE_SSH_DIR", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	assert.NilError(t, exec.CommandContext(t.Context(), filepath.Join(dir, "ssh")).Run())
	return dir
}

func TestSSHMasterStartWaitsUntilReady(t *testing.T) {
	m := newSSHMaster(&ssh.SSHConfig{}, fakeSSHOnPath(t), localSSHAddress)
	defer func() { assert.NilError(t, m.terminate()) }()
	// The requirement sleeps between retries, so the first start waits for
	// a master that is still connecting.
	assert.NilError(t, m.start(t.Context()))
}

func TestSSHMasterReplacesForeignMaster(t *testing.T) {
	dir := fakeSSHOnPath(t)
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "unix", filepath.Join(dir, filenames.SSHSock))
	assert.NilError(t, err)
	defer ln.Close()
	foreign := exec.CommandContext(t.Context(), "sleep", "600")
	assert.NilError(t, foreign.Start())
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "foreign.pid"), []byte(strconv.Itoa(foreign.Process.Pid)), 0o644))

	m := newSSHMaster(&ssh.SSHConfig{}, dir, localSSHAddress)
	defer func() { assert.NilError(t, m.terminate()) }()
	assert.NilError(t, m.start(t.Context()))
	assert.ErrorContains(t, foreign.Wait(), "terminated")
}

func TestSSHMasterTerminateDuringBackoff(t *testing.T) {
	dir := fakeSSHOnPath(t)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "fail"), nil, 0o644))
	m := newSSHMaster(&ssh.SSHConfig{}, dir, localSSHAddress)
	go func() { _ = m.start(t.Context()) }()
	// Wait until the first master has failed and runOnce has dropped it; run
	// enters its backoff right after.
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if _, err := os.Stat(filepath.Join(dir, "failed.pid")); err != nil {
			return poll.Continue("the first master has not failed yet")
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.cmd != nil {
			return poll.Continue("runOnce has not returned yet")
		}
		return poll.Success()
	}, poll.WithDelay(10*time.Millisecond))
	begin := time.Now()
	assert.NilError(t, m.terminate())
	assert.Assert(t, time.Since(begin) < 500*time.Millisecond)
}

func TestSSHMasterKillsPlainConnection(t *testing.T) {
	dir := fakeSSHOnPath(t)
	// Without a listener on the control socket, exitForeignMaster leaves the
	// foreign master alone and our ssh runs as a plain connection.
	foreign := exec.CommandContext(t.Context(), "sleep", "600")
	assert.NilError(t, foreign.Start())
	defer func() { _ = foreign.Process.Kill(); _ = foreign.Wait() }()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "foreign.pid"), []byte(strconv.Itoa(foreign.Process.Pid)), 0o644))

	m := newSSHMaster(&ssh.SSHConfig{}, dir, localSSHAddress)
	defer func() { assert.NilError(t, m.terminate()) }()
	go func() { _ = m.start(t.Context()) }()
	var plain int
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		b, err := os.ReadFile(filepath.Join(dir, "plain.pid"))
		if err != nil || len(b) == 0 {
			return poll.Continue("no plain connection yet")
		}
		plain, err = strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return poll.Error(err)
		}
		return poll.Success()
	}, poll.WithDelay(10*time.Millisecond))
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if processRunning(plain) {
			return poll.Continue("plain connection %d is still running", plain)
		}
		return poll.Success()
	}, poll.WithDelay(10*time.Millisecond), poll.WithTimeout(2*time.Second))
	assert.Assert(t, processRunning(foreign.Process.Pid))
}

func processRunning(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}
