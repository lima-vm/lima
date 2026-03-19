// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// BusyCheck is a named function that reports whether the system is busy.
// If any registered BusyCheck returns true, the idle timer is reset.
type BusyCheck struct {
	Name string
	Fn   func() bool
}

// IdleTracker tracks the last user activity and determines whether
// the VM has been idle for longer than the configured timeout.
type IdleTracker struct {
	mu           sync.Mutex
	lastActivity time.Time
	idleTimeout  time.Duration
	busyChecks   []BusyCheck
}

// NewIdleTracker creates a new IdleTracker with the given idle timeout.
// The tracker starts with the current time as the last activity.
func NewIdleTracker(idleTimeout time.Duration) *IdleTracker {
	return &IdleTracker{
		lastActivity: time.Now(),
		idleTimeout:  idleTimeout,
	}
}

// Touch records user activity, resetting the idle timer.
func (t *IdleTracker) Touch() {
	t.mu.Lock()
	t.lastActivity = time.Now()
	t.mu.Unlock()
}

// IsIdle returns true only if no busy-checks are active AND the idle timeout has elapsed.
func (t *IdleTracker) IsIdle() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Busy-checks take priority: any active check resets the timer.
	for _, check := range t.busyChecks {
		if check.Fn() {
			logrus.Debugf("Idle tracker: busy check %q active, resetting idle timer", check.Name)
			t.lastActivity = time.Now()
			return false
		}
	}

	return time.Since(t.lastActivity) >= t.idleTimeout
}

// IdleDuration returns the duration since the last recorded activity.
func (t *IdleTracker) IdleDuration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return time.Since(t.lastActivity)
}

// AddBusyCheck registers a function that prevents idle detection when it returns true.
// Thread-safe; can be called concurrently with IsIdle().
func (t *IdleTracker) AddBusyCheck(name string, fn func() bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.busyChecks = append(t.busyChecks, BusyCheck{Name: name, Fn: fn})
}
