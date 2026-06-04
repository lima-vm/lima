// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

// startBannerServer starts a TCP listener on 127.0.0.1:0 that emits the given
// banner to each accepted connection and then closes it. If banner is empty,
// the connection is closed immediately (simulating the hostagent-proxies-to-
// dead-guest-sshd race).
func startBannerServer(t *testing.T, banner string) int {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if banner != "" {
					_, _ = c.Write([]byte(banner))
				}
			}(conn)
		}
	}()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	assert.NilError(t, err)
	port, err := strconv.Atoi(portStr)
	assert.NilError(t, err)
	return port
}

func TestProbeSSHBannerOnLocalPort_OK(t *testing.T) {
	port := startBannerServer(t, "SSH-2.0-OpenSSH_9.6p1 Ubuntu-3ubuntu13\r\n")
	assert.NilError(t, probeSSHBannerOnLocalPort(t.Context(), port))
}

func TestProbeSSHBannerOnLocalPort_AnySSHPrefix(t *testing.T) {
	for _, banner := range []string{
		"SSH-1.99-OpenSSH_old\r\n",
		"SSH-2.5-FutureSSH\r\n",
		"SSH-3.0-Hypothetical\r\n",
	} {
		port := startBannerServer(t, banner)
		assert.NilError(t, probeSSHBannerOnLocalPort(t.Context(), port), banner)
	}
}

func TestProbeSSHBannerOnLocalPort_AcceptThenClose(t *testing.T) {
	// Simulates the race: TCP forwarder accepts but the proxied guest peer is
	// not listening, so the host side closes immediately and the client sees EOF.
	port := startBannerServer(t, "")
	err := probeSSHBannerOnLocalPort(t.Context(), port)
	assert.ErrorContains(t, err, "read SSH banner")
}

func TestProbeSSHBannerOnLocalPort_WrongBanner(t *testing.T) {
	port := startBannerServer(t, "HTTP/1.1 200 OK\r\n")
	err := probeSSHBannerOnLocalPort(t.Context(), port)
	assert.ErrorContains(t, err, "unexpected banner")
}

func TestProbeSSHBannerOnLocalPort_NoListener(t *testing.T) {
	// Bind a listener to grab a free port, then close it before probing so the
	// dial returns ECONNREFUSED (or equivalent on the host platform).
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	assert.NilError(t, err)
	port, err := strconv.Atoi(portStr)
	assert.NilError(t, err)
	assert.NilError(t, ln.Close())

	err = probeSSHBannerOnLocalPort(t.Context(), port)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "dial 127.0.0.1:"+portStr))
}

func TestProbeSSHBannerOnLocalPort_HungWrite(t *testing.T) {
	// Server accepts but neither writes a banner nor closes, so the probe must
	// time out on the read deadline rather than hang forever.
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		accepted <- conn
	}()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	assert.NilError(t, err)
	port, err := strconv.Atoi(portStr)
	assert.NilError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 2*probeSSHBannerTimeout)
	defer cancel()
	start := time.Now()
	err = probeSSHBannerOnLocalPort(ctx, port)
	elapsed := time.Since(start)
	if conn := <-accepted; conn != nil {
		_ = conn.Close()
	}
	assert.Assert(t, err != nil)
	assert.Assert(t, elapsed < 2*probeSSHBannerTimeout, "probe did not honor read deadline: elapsed=%s", elapsed)
}
