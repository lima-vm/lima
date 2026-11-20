// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

// Package acceptloop serves the connections that a listener accepts.
package acceptloop

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	minRetryDelay = 5 * time.Millisecond
	maxRetryDelay = time.Second
)

// Serve accepts connections on l and calls handle on each one in a new
// goroutine. It closes l when ctx is done, and returns nil once l is closed.
//
// When Accept fails because the process or the system ran out of a
// resource, such as file descriptors or memory, Serve waits and tries
// again, because the resource frees up once other connections close. Serve
// does the same when one incoming connection failed before Serve accepted
// it, because that failure doesn't affect the connections queued behind it.
// In both cases Serve keeps trying until Accept succeeds or ctx is done.
// The wait starts at 5ms and doubles after each failure until it reaches 1s.
// Serve closes l and returns any other Accept error. Retrying those doesn't
// help. For example, after something else closes a listener's descriptor,
// every later Accept fails with EBADF immediately, and a loop that retries
// fills the log.
func Serve(ctx context.Context, l net.Listener, handle func(net.Conn)) error {
	defer l.Close()
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			l.Close()
		case <-stopped:
		}
	}()
	var delay time.Duration
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			if !isRetryable(err) {
				return err
			}
			delay = min(max(2*delay, minRetryDelay), maxRetryDelay)
			logrus.WithError(err).Warnf("Failed to accept a connection on %s; retrying in %v", l.Addr(), delay)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
			continue
		}
		delay = 0
		go handle(conn)
	}
}

func isRetryable(err error) bool {
	for _, errno := range retryableErrnos {
		if errors.Is(err, errno) {
			return true
		}
	}
	return false
}
