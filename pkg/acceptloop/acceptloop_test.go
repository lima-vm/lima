// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package acceptloop

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"slices"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
)

type acceptResult struct {
	conn net.Conn
	err  error
}

// scriptedListener's Accept returns results in order, then repeats the
// last one.
type scriptedListener struct {
	results []acceptResult
	calls   atomic.Int64
	closed  atomic.Bool
}

func (l *scriptedListener) Accept() (net.Conn, error) {
	n := int(l.calls.Add(1))
	r := l.results[min(n, len(l.results))-1]
	return r.conn, r.err
}

func (l *scriptedListener) Close() error {
	l.closed.Store(true)
	return nil
}

func (l *scriptedListener) Addr() net.Addr { return &net.TCPAddr{} }

func acceptError(errno syscall.Errno) error {
	return &net.OpError{Op: "accept", Net: "tcp", Err: os.NewSyscallError("accept", errno)}
}

func TestServeReturnsAcceptError(t *testing.T) {
	l := &scriptedListener{results: []acceptResult{{err: acceptError(syscall.EBADF)}}}
	done := make(chan error, 1)
	go func() {
		done <- Serve(t.Context(), l, func(net.Conn) {})
	}()
	select {
	case err := <-done:
		assert.Assert(t, errors.Is(err, syscall.EBADF), "err: %v", err)
		assert.Equal(t, l.calls.Load(), int64(1))
		assert.Assert(t, l.closed.Load(), "Serve returned without closing the listener")
	case <-time.After(2 * time.Second):
		assert.Assert(t, false, "Serve did not return after %d Accept calls", l.calls.Load())
	}
}

func TestServeRetriesWhenOutOfDescriptors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c1, c2 := net.Pipe()
		defer c1.Close()
		defer c2.Close()
		outOfDescriptors := acceptResult{err: acceptError(outOfResourceErrnos[0])}
		// Nine failures bring the wait to its 1s maximum. Accepting c1
		// resets the wait, so the failure after c1 waits 5ms again.
		results := slices.Repeat([]acceptResult{outOfDescriptors}, 9)
		results = append(results, acceptResult{conn: c1}, outOfDescriptors, acceptResult{err: net.ErrClosed})
		l := &scriptedListener{results: results}
		hook := logrustest.NewGlobal()
		defer hook.Reset()
		out := logrus.StandardLogger().Out
		logrus.SetOutput(io.Discard)
		defer logrus.SetOutput(out)
		handled := make(chan net.Conn, 2)
		start := time.Now()
		err := Serve(t.Context(), l, func(c net.Conn) { handled <- c })
		elapsed := time.Since(start)
		assert.NilError(t, err)
		assert.Equal(t, <-handled, c1)
		assert.Equal(t, len(handled), 0)
		assert.Equal(t, l.calls.Load(), int64(len(results)))
		var warnings []string
		for _, e := range hook.AllEntries() {
			assert.Equal(t, e.Level, logrus.WarnLevel)
			warnings = append(warnings, e.Message)
		}
		var want []string
		for _, d := range []string{"5ms", "10ms", "20ms", "40ms", "80ms", "160ms", "320ms", "640ms", "1s", "5ms"} {
			want = append(want, "Failed to accept a connection on :0; retrying in "+d)
		}
		assert.DeepEqual(t, warnings, want)
		assert.Equal(t, elapsed, 2280*time.Millisecond)
	})
}

func TestServeStopsWhenContextIsDone(t *testing.T) {
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, l, func(c net.Conn) { c.Close() })
	}()
	cancel()
	select {
	case err := <-done:
		assert.NilError(t, err)
	case <-time.After(2 * time.Second):
		assert.Assert(t, false, "Serve did not return after the context was canceled")
	}
	_, err = l.Accept()
	assert.Assert(t, errors.Is(err, net.ErrClosed), "err: %v", err)
}
