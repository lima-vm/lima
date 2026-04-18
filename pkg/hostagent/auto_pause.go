// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package hostagent

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/driver"
	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

// AutoPauseManager monitors VM idle state and pauses/resumes the VM automatically.
// It watches an IdleTracker and calls Pause on the driver when the VM is idle,
// and Resume when activity is detected via Touch().
type AutoPauseManager struct {
	pausable      driver.Pausable
	idleTracker   *IdleTracker
	resumeTimeout time.Duration
	resumeCh      chan struct{} // signaled when Touch() is called while paused
	tickInterval  time.Duration // polling interval for idle check
	callbackMu    sync.Mutex    // protects onResume, onWake, and socketProxy from concurrent access
	onResume      func()        // called after successful Resume(), may be nil
	onWake        func()        // called when system wake is detected (VM must be running)
	lastTick      time.Time     // tracks last tick time for wake detection
	metricsMu     sync.Mutex
	latestMetrics *api.MemoryMetrics
	metricsTime   time.Time
	prevIOBytes   float64             // previous IO reading for delta detection
	signalConfig  IdleSignalConfig    // resolved idle-signal configuration (immutable after construction)
	lastDeferLog  time.Time           // throttles deferred-pause info logging
	socketProxy   *SocketForwardProxy // set during wiring, used for throttled logging
	forcePauseCh  chan chan error     // channel for ForcePause requests
	done          chan struct{}       // closed when Run() exits
}

// IdleSignalConfig holds the resolved idle-signal configuration.
// All defaults have been applied; no pointer fields.
type IdleSignalConfig struct {
	ActiveConnections     bool
	ContainerCPU          bool
	ContainerCPUThreshold float64
	ContainerIO           bool
}

// DefaultIdleSignalConfig returns the default configuration (all signals enabled).
func DefaultIdleSignalConfig() IdleSignalConfig {
	return IdleSignalConfig{
		ActiveConnections:     true,
		ContainerCPU:          true,
		ContainerCPUThreshold: 0.5,
		ContainerIO:           true,
	}
}

// NewAutoPauseManager creates a new AutoPauseManager with the given signal configuration.
func NewAutoPauseManager(
	pausable driver.Pausable,
	idleTimeout, resumeTimeout time.Duration,
	signalConfig IdleSignalConfig,
) *AutoPauseManager {
	return &AutoPauseManager{
		pausable:      pausable,
		idleTracker:   NewIdleTracker(idleTimeout),
		resumeTimeout: resumeTimeout,
		resumeCh:      make(chan struct{}, 1),
		// 1s tick provides responsive pause detection and fast retry on resume failure,
		// while keeping CPU overhead negligible (single IsPaused() + IsIdle() check per tick).
		tickInterval: 1 * time.Second,
		signalConfig: signalConfig,
		lastDeferLog: time.Now(), // suppress first log for 1 minute to let system settle
		forcePauseCh: make(chan chan error, 1),
		done:         make(chan struct{}),
	}
}

// Touch records user activity. If the VM is currently paused, it triggers a resume.
// Multiple rapid calls are coalesced to a single resume attempt via a buffered channel.
func (m *AutoPauseManager) Touch() {
	m.idleTracker.Touch()
	select {
	case m.resumeCh <- struct{}{}:
	default:
	}
}

// IsPaused returns true if the VM is currently paused.
func (m *AutoPauseManager) IsPaused() bool {
	return m.pausable.IsPaused()
}

// ForcePause requests an immediate pause of the VM.
// It blocks until the pause completes or ctx is cancelled.
// If the VM is already paused, it returns nil immediately.
// Returns an error if the Run() loop has already exited.
func (m *AutoPauseManager) ForcePause(ctx context.Context) error {
	errCh := make(chan error, 1)
	select {
	case m.forcePauseCh <- errCh:
	case <-m.done:
		return errors.New("auto-pause manager is not running")
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-errCh:
		return err
	case <-m.done:
		return errors.New("auto-pause manager is not running")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AddBusyCheck registers a function that prevents idle detection when it returns true.
// Thread-safe; can be called before or concurrently with Run().
func (m *AutoPauseManager) AddBusyCheck(name string, fn func() bool) {
	m.idleTracker.AddBusyCheck(name, fn)
}

// WaitForRunning blocks until the VM is not paused or ctx is cancelled.
// Returns nil when the VM is running, or ctx.Err() if cancelled.
func (m *AutoPauseManager) WaitForRunning(ctx context.Context) error {
	if !m.pausable.IsPaused() {
		return nil
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for m.pausable.IsPaused() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return nil
}

// UpdateGuestMetrics receives fresh metrics from the balloon controller poll loop.
func (m *AutoPauseManager) UpdateGuestMetrics(metrics *api.MemoryMetrics) {
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()
	m.latestMetrics = metrics
	m.metricsTime = time.Now()
}

// hasContainerCPUActivity returns true if running containers show measurable CPU.
// Called from IdleTracker.IsIdle() as a BusyCheck — must NOT acquire IdleTracker.mu.
func (m *AutoPauseManager) hasContainerCPUActivity() bool {
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()

	if m.latestMetrics == nil || time.Since(m.metricsTime) > 30*time.Second {
		return false
	}
	if m.latestMetrics.ContainerCount <= 0 {
		return false
	}
	return m.latestMetrics.ContainerCpuPercent > m.signalConfig.ContainerCPUThreshold
}

// hasContainerIOActivity returns true if running containers show changing IO.
// Called from IdleTracker.IsIdle() as a BusyCheck in the Run() goroutine only.
// Mutates prevIOBytes under metricsMu — must NOT be called from other goroutines.
func (m *AutoPauseManager) hasContainerIOActivity() bool {
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()

	if m.latestMetrics == nil || time.Since(m.metricsTime) > 30*time.Second {
		return false
	}
	if m.latestMetrics.ContainerCount <= 0 {
		return false
	}

	currentIO := m.latestMetrics.ContainerIoBytesPerSec
	if math.Abs(currentIO-m.prevIOBytes) > 1.0 {
		m.prevIOBytes = currentIO
		return true
	}
	return false
}

// resumeVM resumes the VM with the configured resume timeout.
func (m *AutoPauseManager) resumeVM(ctx context.Context, reason string) {
	if !m.pausable.IsPaused() {
		return
	}
	logrus.Infof("Auto-pause: resuming VM (%s)", reason)
	resumeCtx, cancel := context.WithTimeout(ctx, m.resumeTimeout)
	defer cancel()
	if err := m.pausable.Resume(resumeCtx); err != nil {
		logrus.WithError(err).Errorf("Auto-pause: failed to resume VM (%s)", reason)
		return
	}
	m.callbackMu.Lock()
	fn := m.onResume
	m.callbackMu.Unlock()
	if fn != nil {
		fn()
	}
}

// checkWake detects system wake by comparing the current time to lastTick.
// If the gap exceeds 5s, a wake event is detected. Returns true if wake was
// detected. Updates lastTick to now.
func (m *AutoPauseManager) checkWake(now time.Time) bool {
	elapsed := now.Sub(m.lastTick)
	m.lastTick = now

	if elapsed <= 5*time.Second {
		return false
	}

	logrus.Infof("Auto-pause: detected system wake (gap %s)", elapsed.Round(time.Second))
	m.idleTracker.Touch() // reset idle timer (don't pause right after wake)

	// Only refresh tunnels if VM is running. If paused, let the next client
	// connection trigger resume + tunnel refresh via handleConn().
	if !m.pausable.IsPaused() {
		m.callbackMu.Lock()
		fn := m.onWake
		m.callbackMu.Unlock()
		if fn != nil {
			fn()
		}
	}
	return true
}

// Run starts the auto-pause loop. It blocks until ctx is cancelled.
func (m *AutoPauseManager) Run(ctx context.Context) {
	defer close(m.done)
	ticker := time.NewTicker(m.tickInterval)
	defer ticker.Stop()
	m.lastTick = time.Now() // prevents false positive wake detection on first tick

	for {
		select {
		case <-ctx.Done():
			// On shutdown, ensure VM is resumed if paused. Use a fresh context
			// since the parent context is already cancelled.
			if m.pausable.IsPaused() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), m.resumeTimeout)
				logrus.Info("Auto-pause: resuming VM on shutdown")
				if err := m.pausable.Resume(shutdownCtx); err != nil {
					logrus.WithError(err).Warn("Auto-pause: failed to resume VM on shutdown")
				}
				cancel()
			}
			return

		case <-m.resumeCh:
			m.resumeVM(ctx, "activity detected")

		case errCh := <-m.forcePauseCh:
			if m.pausable.IsPaused() {
				errCh <- nil
			} else {
				logrus.Info("Auto-pause: manual pause requested")
				errCh <- m.pausable.Pause(ctx)
			}

		case <-ticker.C:
			m.checkWake(time.Now())

			// Best-effort throttled log when busy-checks keep VM awake.
			if !m.pausable.IsPaused() && time.Since(m.lastDeferLog) > 1*time.Minute {
				logged := false

				// Only check proxy connections if signal is enabled.
				if !logged && m.signalConfig.ActiveConnections {
					m.callbackMu.Lock()
					p := m.socketProxy
					m.callbackMu.Unlock()
					if p != nil && p.HasActiveConnections() {
						logrus.Infof("Auto-pause: %d active proxy connections keeping VM awake",
							p.ActiveConnectionCount())
						logged = true
					}
				}

				// Check container CPU if enabled (read-only — no prevIOBytes mutation).
				if !logged && m.signalConfig.ContainerCPU {
					m.metricsMu.Lock()
					hasActivity := m.latestMetrics != nil &&
						m.latestMetrics.ContainerCount > 0 &&
						m.latestMetrics.ContainerCpuPercent > m.signalConfig.ContainerCPUThreshold
					m.metricsMu.Unlock()
					if hasActivity {
						logrus.Infof("Auto-pause: container CPU activity keeping VM awake")
						logged = true
					}
				}

				// Check container IO if enabled (read-only peek — no prevIOBytes mutation).
				if !logged && m.signalConfig.ContainerIO {
					m.metricsMu.Lock()
					ioActive := m.latestMetrics != nil &&
						m.latestMetrics.ContainerCount > 0 &&
						math.Abs(m.latestMetrics.ContainerIoBytesPerSec-m.prevIOBytes) > 1.0
					m.metricsMu.Unlock()
					if ioActive {
						logrus.Infof("Auto-pause: container IO activity keeping VM awake")
						logged = true
					}
				}

				if logged {
					m.lastDeferLog = time.Now()
				}
			}

			if m.idleTracker.IsIdle() && !m.pausable.IsPaused() {
				logrus.Infof("Auto-pause: VM idle for %s, pausing", m.idleTracker.IdleDuration().Round(time.Second))
				if err := m.pausable.Pause(ctx); err != nil {
					logrus.WithError(err).Error("Auto-pause: failed to pause VM")
				}
			}
		}
	}
}
