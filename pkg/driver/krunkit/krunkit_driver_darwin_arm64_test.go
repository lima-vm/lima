// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krunkit

import (
	"io"
	"net"
	"net/http"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/limatype"
)

func TestValidateConfigRejectsBlockDevices(t *testing.T) {
	err := validateConfig(&limatype.LimaYAML{BlockDevices: []string{"/dev/disk4"}})
	assert.ErrorContains(t, err, "field `blockDevices` is not supported for vmType: krunkit")
}

func TestValidateRejectsHostLoopbackFalse(t *testing.T) {
	// krunkit's netstack does not honor the option, so the refusal is all that stops a silent no-op.
	l := &LimaKrunkitDriver{Instance: &limatype.Instance{Config: &limatype.LimaYAML{NetworkOpts: limatype.NetworkOpts{HostLoopback: new(false)}}}}
	assert.ErrorContains(t, l.Validate(t.Context()), "`networkOpts.hostLoopback: false` is not supported in krunkit driver")
}

func serveUnix(t *testing.T, reply string) string {
	t.Helper()
	t.Chdir(t.TempDir())
	sock := "s.sock"
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "unix", sock)
	assert.NilError(t, err)
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte(reply))
			conn.Close()
		}
	}()
	return sock
}

func TestReadVsockSSHBanner(t *testing.T) {
	sock := serveUnix(t, "SSH-2.0-OpenSSH_10.0\r\n")
	assert.NilError(t, readVsockSSHBanner(t.Context(), sock))
}

func TestReadVsockSSHBannerAfterPreamble(t *testing.T) {
	sock := serveUnix(t, "Welcome\r\nAuthorized use only\r\nSSH-2.0-OpenSSH_10.0\r\n")
	assert.NilError(t, readVsockSSHBanner(t.Context(), sock))
}

func TestReadVsockSSHBannerClosedWithoutBanner(t *testing.T) {
	// krunkit accepts the host socket and then closes it when nothing listens in the guest.
	sock := serveUnix(t, "")
	assert.ErrorIs(t, readVsockSSHBanner(t.Context(), sock), io.EOF)
}

// serveKrunkitREST serves a stand-in for krunkit's REST service on restfulSock in the current
// directory. It sends each request it receives on the returned channel, answers with status, and
// then calls onRequest, if set.
func serveKrunkitREST(t *testing.T, status int, onRequest func()) <-chan string {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "unix", restfulSock)
	assert.NilError(t, err)
	reqs := make(chan string, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			reqs <- r.Method + " " + r.URL.Path + " " + string(body)
			w.WriteHeader(status)
			if onRequest != nil {
				onRequest()
			}
		}),
		ReadHeaderTimeout: time.Second,
	}
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Close() })
	return reqs
}

func TestRequestGuestStop(t *testing.T) {
	t.Chdir(t.TempDir())
	reqs := serveKrunkitREST(t, http.StatusOK, nil)
	assert.NilError(t, requestGuestStop(t.Context(), restfulSock))
	assert.Equal(t, <-reqs, `POST /vm/state {"state":"Stop"}`)
}

func TestRequestGuestStopRejected(t *testing.T) {
	t.Chdir(t.TempDir())
	serveKrunkitREST(t, http.StatusBadRequest, nil)
	assert.ErrorContains(t, requestGuestStop(t.Context(), restfulSock), "400 Bad Request")
}

// startFakeKrunkit starts a process that stands in for krunkit and returns a driver that runs it.
func startFakeKrunkit(t *testing.T) *LimaKrunkitDriver {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "sleep", "60")
	assert.NilError(t, cmd.Start())
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()
	return &LimaKrunkitDriver{Instance: &limatype.Instance{Dir: "."}, krunkitCmd: cmd, krunkitWaitCh: waitCh}
}

func exitSignal(t *testing.T, cmd *exec.Cmd) syscall.Signal {
	t.Helper()
	ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	assert.Assert(t, ok)
	assert.Assert(t, ws.Signaled())
	return ws.Signal()
}

func TestStopAsksGuestThroughREST(t *testing.T) {
	t.Chdir(t.TempDir())
	l := startFakeKrunkit(t)
	// krunkit exits once the guest has shut down; SIGKILL stands for that exit here.
	reqs := serveKrunkitREST(t, http.StatusOK, func() { _ = l.krunkitCmd.Process.Kill() })
	assert.NilError(t, l.Stop(t.Context()))
	assert.Equal(t, <-reqs, `POST /vm/state {"state":"Stop"}`)
	assert.Equal(t, exitSignal(t, l.krunkitCmd), syscall.SIGKILL, "krunkit must not get SIGTERM when the REST request succeeds")
}

func TestStopFallsBackToSIGTERM(t *testing.T) {
	t.Chdir(t.TempDir())
	l := startFakeKrunkit(t) // no REST service listening
	assert.NilError(t, l.Stop(t.Context()))
	assert.Equal(t, exitSignal(t, l.krunkitCmd), syscall.SIGTERM)
}
