//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	hostagentclient "github.com/lima-vm/lima/v2/pkg/hostagent/api/client"
)

// shortTempDir returns a temp dir short enough to hold a unix socket: the sockaddr_un path
// limit (104 bytes on macOS) is well below the length that t.TempDir() produces.
func shortTempDir(t *testing.T) string {
	t.Helper()
	//nolint:usetesting // t.TempDir() returns a path too long for a unix socket.
	dir, err := os.MkdirTemp("", "lima")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// listenUnix starts a unix listener that keeps its socket file when closed, the way a host
// agent that was killed would leave it behind.
func listenUnix(t *testing.T, sock string) *net.UnixListener {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "unix", sock)
	assert.NilError(t, err)
	ul, ok := l.(*net.UnixListener)
	assert.Assert(t, ok, "expected a *net.UnixListener")
	ul.SetUnlinkOnClose(false)
	return ul
}

// dialInfo makes the same call that Inspect makes against a host agent socket, so the errors
// under test are the ones the real client produces rather than synthetic values.
func dialInfo(t *testing.T, sock string, timeout time.Duration) error {
	t.Helper()
	cli, err := hostagentclient.NewHostAgentClient(sock)
	assert.NilError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	_, err = cli.Info(ctx)
	assert.Assert(t, err != nil, "expected Info to fail")
	return err
}

// A socket file that exists with nothing listening on it is what a departed host agent leaves
// behind, and the only case that may be reported as recoverable.
func TestMarkStaleHostAgentRefusedConnection(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "ha.sock")
	assert.NilError(t, listenUnix(t, sock).Close())
	_, err := os.Stat(sock)
	assert.NilError(t, err, "the socket file must survive the listener")

	got := markStaleHostAgent(dialInfo(t, sock, 3*time.Second))
	assert.Assert(t, errors.Is(got, ErrHostAgentUnreachable),
		"a refused connection must be marked recoverable, got: %v", got)
}

// A host agent that is alive but too slow to answer must not be reported as recoverable:
// acting on the sentinel would delete a live agent's files and start a second one.
func TestMarkStaleHostAgentTimeoutIsNotStale(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "ha.sock")
	l := listenUnix(t, sock)
	defer l.Close()
	// Accept the connection but never reply, so the client's context expires instead.
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		<-t.Context().Done()
		_ = conn.Close()
	}()

	err := dialInfo(t, sock, 100*time.Millisecond)
	assert.Assert(t, errors.Is(err, context.DeadlineExceeded), "expected a timeout, got: %v", err)
	got := markStaleHostAgent(err)
	assert.Assert(t, !errors.Is(got, ErrHostAgentUnreachable),
		"a live but slow host agent must not be marked recoverable, got: %v", got)
}

// A socket that does not exist yet is the startup race: the host agent has written its PID
// file but has not bound the socket. Inspect reports that error without the sentinel; this
// covers markStaleHostAgent directly so the intent survives any future refactoring.
func TestMarkStaleHostAgentMissingSocketIsNotStale(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "ha.sock")
	_, err := hostagentclient.NewHostAgentClient(sock)
	assert.Assert(t, err != nil, "expected the client to reject a missing socket")
	assert.Assert(t, !errors.Is(markStaleHostAgent(err), ErrHostAgentUnreachable),
		"a socket that does not exist yet must not be marked recoverable")
}
