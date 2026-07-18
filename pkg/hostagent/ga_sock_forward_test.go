// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lima-vm/sshocker/pkg/ssh"
	"gotest.tools/v3/assert"
)

// Regression test for https://github.com/lima-vm/lima/issues/2227.
//
// Two invariants must hold for the guest-agent socket forward:
//  1. forwardGuestAgentSock must issue verbCancel before verbForward on every
//     call, so the SSH ControlMaster releases the prior registration and the
//     new bind succeeds cleanly. Otherwise forwardSSH unlinks the local
//     ga.sock and the socket disappears from disk until limactl stop/start.
//  2. gaSockForwardMu must serialize the forward path against the cancel
//     path (cleanup) and against concurrent reconnect ticks. Without
//     serialization, os.RemoveAll/bind races leave ga.sock missing.

type forwardCall struct {
	verb  string
	local string
}

// stubForwardSSH replaces the package-level forwardSSH with a stub that
// records calls and optionally runs hook on every call. It returns an
// accessor for the recorded calls and a restore function.
func stubForwardSSH(t *testing.T, hook func(verb string)) (calls func() []forwardCall, restore func()) {
	t.Helper()
	var mu sync.Mutex
	var recorded []forwardCall
	orig := forwardSSH
	forwardSSH = func(_ context.Context, _ *ssh.SSHConfig, _ string, _ int, local, _, verb string, _ bool) error {
		if hook != nil {
			hook(verb)
		}
		mu.Lock()
		recorded = append(recorded, forwardCall{verb: verb, local: local})
		mu.Unlock()
		return nil
	}
	return func() []forwardCall {
			mu.Lock()
			defer mu.Unlock()
			out := make([]forwardCall, len(recorded))
			copy(out, recorded)
			return out
		}, func() {
			forwardSSH = orig
		}
}

func TestForwardGuestAgentSock_CancelsBeforeForward(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	a := &HostAgent{}
	a.forwardGuestAgentSock(t.Context(), "/tmp/ga.sock", "/run/lima-guestagent.sock")

	got := calls()
	assert.Equal(t, len(got), 2, "expected exactly one cancel and one forward")
	assert.Equal(t, got[0].verb, verbCancel, "cancel must come before forward")
	assert.Equal(t, got[1].verb, verbForward, "forward must follow cancel")
}

func TestCancelGuestAgentSockForward_IssuesCancel(t *testing.T) {
	calls, restore := stubForwardSSH(t, nil)
	defer restore()

	a := &HostAgent{}
	err := a.cancelGuestAgentSockForward(t.Context(), "/tmp/ga.sock", "/run/lima-guestagent.sock")
	assert.NilError(t, err)

	got := calls()
	assert.Equal(t, len(got), 1)
	assert.Equal(t, got[0].verb, verbCancel)
}

// TestGuestAgentSockForward_SerializedUnderContention runs many concurrent
// forward and cancel callers and asserts that at most one forwardSSH call is
// in flight at any moment (max observed concurrency == 1). If
// gaSockForwardMu were missing or scoped incorrectly, two goroutines could
// enter forwardSSH simultaneously and race on the local ga.sock listener.
func TestGuestAgentSockForward_SerializedUnderContention(t *testing.T) {
	const (
		forwarders = 8
		cancelers  = 8
		iterations = 50
	)

	var (
		mu        sync.Mutex
		active    int
		maxActive int
	)
	hook := func(_ string) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		// Widen the window so any missing serialization can manifest.
		time.Sleep(50 * time.Microsecond)
		mu.Lock()
		active--
		mu.Unlock()
	}

	_, restore := stubForwardSSH(t, hook)
	defer restore()

	a := &HostAgent{}
	var wg sync.WaitGroup
	for range forwarders {
		wg.Go(func() {
			for range iterations {
				a.forwardGuestAgentSock(t.Context(), "/tmp/ga.sock", "/run/lima-guestagent.sock")
			}
		})
	}
	for range cancelers {
		wg.Go(func() {
			for range iterations {
				_ = a.cancelGuestAgentSockForward(t.Context(), "/tmp/ga.sock", "/run/lima-guestagent.sock")
			}
		})
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, maxActive, 1, "forwardSSH calls overlapped — gaSockForwardMu is not serializing the ga.sock forward path")
}
