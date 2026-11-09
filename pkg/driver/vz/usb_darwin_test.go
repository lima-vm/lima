//go:build darwin && !no_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/poll"

	"github.com/lima-vm/lima/v2/pkg/usbip"
)

// fakeListener returns an Accept error for a nil conn, as vz does.
type fakeListener struct {
	net.Listener // only Accept is used
	conns        chan net.Conn
}

func (l *fakeListener) Accept() (net.Conn, error) {
	c := <-l.conns
	if c == nil {
		return nil, errors.New("accept failed")
	}
	return c, nil
}

func TestAcceptUSBIP(t *testing.T) {
	hook := logrustest.NewGlobal()
	warns := func() int {
		n := 0
		for _, e := range hook.AllEntries() {
			if e.Level == logrus.WarnLevel {
				n++
			}
		}
		return n
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	l := &fakeListener{conns: make(chan net.Conn)}
	done := make(chan struct{})
	go func() {
		acceptUSBIP(ctx, l, usbip.NewServer(nil))
		close(done)
	}()

	push := func(c net.Conn) {
		select {
		case l.conns <- c:
		case <-time.After(5 * time.Second):
			assert.Assert(t, false, "accept loop stopped")
		}
	}

	baseline := runtime.NumGoroutine()
	push(nil)       // vz accept error: warn, keep accepting
	for range 100 { // guests hanging up is no failure, and their sessions end
		c, s := net.Pipe()
		push(s)
		assert.NilError(t, c.Close())
	}
	deadline := time.Now().Add(5 * time.Second) // not poll: it checks in a goroutine of its own
	for runtime.NumGoroutine() > baseline && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Assert(t, runtime.NumGoroutine() <= baseline, "goroutines %d, baseline %d", runtime.NumGoroutine(), baseline)

	bad, srv := net.Pipe()
	push(srv)
	_, err := bad.Write([]byte{0x01, 0x11, 0xff, 0xff, 0, 0, 0, 0})
	assert.NilError(t, err)
	poll.WaitOn(t, func(poll.LogT) poll.Result {
		if warns() >= 2 {
			return poll.Success()
		}
		return poll.Continue("%d warnings", warns())
	}, poll.WithDelay(5*time.Millisecond))

	idle, srv := net.Pipe()
	assert.NilError(t, idle.SetReadDeadline(time.Now().Add(5*time.Second)))
	push(srv)
	cancel()
	_, err = idle.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)
	close(l.conns)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		assert.Assert(t, false, "accept loop did not stop")
	}
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, warns(), 2, "VM stop is no failure")
}

func TestAcceptUSBIPCapsConnections(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	l := &fakeListener{conns: make(chan net.Conn)}
	go acceptUSBIP(ctx, l, usbip.NewServer(nil))
	t.Cleanup(func() { close(l.conns) })

	idle := make([]net.Conn, maxUSBIPConns)
	for i := range idle {
		var srv net.Conn
		idle[i], srv = net.Pipe()
		l.conns <- srv
	}
	over, srv := net.Pipe()
	assert.NilError(t, over.SetReadDeadline(time.Now().Add(5*time.Second)))
	l.conns <- srv
	_, err := over.Read(make([]byte, 1))
	assert.ErrorIs(t, err, io.EOF)

	// A free slot takes a new conn: EOF would come at once if it were dropped.
	assert.NilError(t, idle[0].Close())
	deadline := time.Now().Add(5 * time.Second)
	for {
		next, srv := net.Pipe()
		assert.NilError(t, next.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
		l.conns <- srv
		_, err := next.Read(make([]byte, 1))
		if errors.Is(err, os.ErrDeadlineExceeded) {
			break
		}
		assert.Assert(t, time.Now().Before(deadline), "slot never freed: %v", err)
	}
}
