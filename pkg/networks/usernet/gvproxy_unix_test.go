//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package usernet

import (
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/balajiv113/fd"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"
)

func TestListenFDClosesConnectionWithoutSocketFD(t *testing.T) {
	//nolint:usetesting // t.TempDir() may return a path too long for a Unix socket.
	dir, err := os.MkdirTemp("", "lima")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	savedOpts := opts
	t.Cleanup(func() { opts = savedOpts })
	opts = &GVisorNetstackOpts{FdSocket: filepath.Join(dir, "fd.sock")}
	hook := logrustest.NewGlobal()
	t.Cleanup(hook.Reset)
	out := logrus.StandardLogger().Out
	logrus.SetOutput(io.Discard)
	t.Cleanup(func() { logrus.SetOutput(out) })
	assert.NilError(t, listenFD(t.Context(), nil))

	pipe, pipeWriter, err := os.Pipe()
	assert.NilError(t, err)
	t.Cleanup(func() {
		_ = pipe.Close()
		_ = pipeWriter.Close()
	})
	// The first client sends a byte and the second passes a pipe, so listenFD
	// doesn't get a socket descriptor from either. It must close each
	// connection and keep accepting new ones.
	clients := []func(*net.UnixConn) error{
		func(conn *net.UnixConn) error {
			_, err := conn.Write([]byte{0})
			return err
		},
		func(conn *net.UnixConn) error {
			return fd.Put(conn, pipe)
		},
	}
	for _, send := range clients {
		var dialer net.Dialer
		conn, err := dialer.DialContext(t.Context(), "unix", opts.FdSocket)
		assert.NilError(t, err)
		assert.NilError(t, send(conn.(*net.UnixConn)))
		assert.NilError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		_, err = conn.Read(make([]byte, 1))
		assert.Assert(t, err != nil && !errors.Is(err, os.ErrDeadlineExceeded), "listenFD did not close the connection: %v", err)
		_ = conn.Close()
	}
	// listenFD closes each connection before it logs the error, so the
	// entries may not be there yet when Read returns.
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if n := len(hook.AllEntries()); n < len(clients) {
			return poll.Continue("%d of %d errors logged", n, len(clients))
		}
		return poll.Success()
	})
	var logged []string
	for _, entry := range hook.AllEntries() {
		logged = append(logged, entry.Message)
	}
	slices.Sort(logged)
	assert.DeepEqual(t, logged, []string{"Error in FD Socket", "Failed to get FD via socket"})
}
