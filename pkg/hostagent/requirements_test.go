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

const testHost = "127.0.0.1"

// startBannerServer listens on 127.0.0.1:0 and writes banner to each accepted
// connection before closing it. An empty banner triggers immediate close.
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
	assert.NilError(t, probeSSHBannerOnLocalPort(t.Context(), testHost, port))
}

func TestProbeSSHBannerOnLocalPort_AnySSHPrefix(t *testing.T) {
	for _, banner := range []string{
		"SSH-1.99-OpenSSH_old\r\n",
		"SSH-2.5-FutureSSH\r\n",
		"SSH-3.0-Hypothetical\r\n",
	} {
		port := startBannerServer(t, banner)
		assert.NilError(t, probeSSHBannerOnLocalPort(t.Context(), testHost, port), banner)
	}
}

// RFC 4253 §4.2: server MAY send lines of data before the identification string.
func TestProbeSSHBannerOnLocalPort_WithPreamble(t *testing.T) {
	port := startBannerServer(t, "Welcome to ExampleHost\r\nLegal notice\r\nSSH-2.0-OpenSSH_9.6p1\r\n")
	assert.NilError(t, probeSSHBannerOnLocalPort(t.Context(), testHost, port))
}

func TestProbeSSHBannerOnLocalPort_AcceptThenClose(t *testing.T) {
	port := startBannerServer(t, "")
	err := probeSSHBannerOnLocalPort(t.Context(), testHost, port)
	assert.ErrorContains(t, err, "read SSH banner")
}

func TestProbeSSHBannerOnLocalPort_NoSSHLine(t *testing.T) {
	port := startBannerServer(t, "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")
	err := probeSSHBannerOnLocalPort(t.Context(), testHost, port)
	assert.Assert(t, err != nil)
}

func TestProbeSSHBannerOnLocalPort_NoListener(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	assert.NilError(t, err)
	port, err := strconv.Atoi(portStr)
	assert.NilError(t, err)
	assert.NilError(t, ln.Close())

	err = probeSSHBannerOnLocalPort(t.Context(), testHost, port)
	assert.Assert(t, err != nil)
	assert.Assert(t, strings.Contains(err.Error(), "dial 127.0.0.1:"+portStr))
}

// Server accepts but never writes nor closes; probe must honor its read deadline.
func TestProbeSSHBannerOnLocalPort_HungWrite(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	assert.NilError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-t.Context().Done()
	}()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	assert.NilError(t, err)
	port, err := strconv.Atoi(portStr)
	assert.NilError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 2*probeSSHBannerTimeout)
	defer cancel()
	start := time.Now()
	err = probeSSHBannerOnLocalPort(ctx, testHost, port)
	elapsed := time.Since(start)
	assert.Assert(t, err != nil)
	assert.Assert(t, elapsed < 2*probeSSHBannerTimeout, "probe did not honor read deadline: elapsed=%s", elapsed)
}
