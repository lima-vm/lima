// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package metrics

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

const pageSize = 4096 // Linux page size in bytes.

// Collector is a stateful memory metrics collector that tracks deltas
// between successive /proc/vmstat samples and reads cgroupfs for
// container activity. It is goroutine-safe.
type Collector struct {
	mu sync.Mutex

	// Previous vmstat sample for delta computation.
	prevVmstat     vmstatCounters
	prevVmstatTime time.Time
	hasPrevVmstat  bool

	// Computed rates (updated by computeVmstatDeltas).
	swapInBytesPerSec  float64
	swapOutBytesPerSec float64
	pageFaultRate      float64
	oomDetected        bool

	// Previous per-container cgroup CPU/IO samples for rate computation.
	prevCgroupCPU  map[string]uint64 // path → usage_usec
	prevCgroupIO   map[string]uint64 // path → total bytes
	prevCgroupTime time.Time
}

// NewCollector creates a new stateful metrics collector.
func NewCollector() *Collector {
	return &Collector{
		prevCgroupCPU: make(map[string]uint64),
		prevCgroupIO:  make(map[string]uint64),
	}
}

// computeVmstatDeltas updates the collector's rate fields from a new
// vmstat sample. Must be called with c.mu held.
func (c *Collector) computeVmstatDeltas(vs vmstatCounters, now time.Time) {
	if !c.hasPrevVmstat {
		c.prevVmstat = vs
		c.prevVmstatTime = now
		c.hasPrevVmstat = true
		return
	}

	dt := now.Sub(c.prevVmstatTime).Seconds()
	if dt <= 0 {
		return
	}

	// Compute deltas; treat counter decreases (reboot) as zero.
	swapInDelta := safeDelta(vs.pswpin, c.prevVmstat.pswpin)
	swapOutDelta := safeDelta(vs.pswpout, c.prevVmstat.pswpout)
	pgfaultDelta := safeDelta(vs.pgfault, c.prevVmstat.pgfault)
	oomDelta := safeDelta(vs.oomKill, c.prevVmstat.oomKill)

	c.swapInBytesPerSec = float64(swapInDelta) * pageSize / dt
	c.swapOutBytesPerSec = float64(swapOutDelta) * pageSize / dt
	c.pageFaultRate = float64(pgfaultDelta) / dt

	if oomDelta > 0 {
		c.oomDetected = true
	}

	c.prevVmstat = vs
	c.prevVmstatTime = now
}

// consumeOomDetected returns and clears the OOM detected flag.
// This implements edge-triggered semantics: the flag is set when a new
// OOM kill is detected and cleared after being read once.
func (c *Collector) consumeOomDetected() bool {
	val := c.oomDetected
	c.oomDetected = false
	return val
}

// safeDelta returns curr - prev if curr >= prev, else 0.
func safeDelta(curr, prev uint64) uint64 {
	if curr >= prev {
		return curr - prev
	}
	return 0
}

// Collect gathers all memory metrics and returns a MemoryMetrics protobuf.
// This is the main entry point called by the guest agent gRPC server.
func (c *Collector) Collect(ctx context.Context) (*api.MemoryMetrics, error) {
	// 1. /proc/meminfo + /proc/pressure/memory (no lock needed — pure reads).
	meminfo, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc/meminfo: %w", err)
	}
	m, err := parseProcMeminfo(meminfo)
	if err != nil {
		return nil, err
	}

	pressure, _ := os.ReadFile("/proc/pressure/memory")
	psi, parseErr := parseProcPressureMemory(pressure)
	if parseErr != nil {
		return nil, parseErr
	}
	m.PsiMemorySome_10 = psi.Some10
	m.PsiMemoryFull_10 = psi.Full10
	m.PsiMemorySome_60 = psi.Some60
	m.PsiMemoryFull_60 = psi.Full60

	// 2. /proc/vmstat for swap rates, page faults, OOM.
	vmstatData, vmstatErr := os.ReadFile("/proc/vmstat")

	// 3. Container stats from cgroupfs (best-effort).
	now := time.Now()
	cgroupPaths := containerCgroupPaths()
	containerCount := len(cgroupPaths)
	var containerCPU, containerIO float64

	// Hold lock for internal state updates.
	c.mu.Lock()
	defer c.mu.Unlock()

	if vmstatErr == nil {
		vs, parseErr := parseProcVmstat(vmstatData)
		if parseErr == nil {
			c.computeVmstatDeltas(vs, now)
		}
	}
	m.SwapInBytesPerSec = c.swapInBytesPerSec
	m.SwapOutBytesPerSec = c.swapOutBytesPerSec
	m.PageFaultRate = c.pageFaultRate
	m.OomDetected = c.consumeOomDetected()

	// Compute container CPU% and IO rates from cgroup deltas.
	if containerCount > 0 {
		containerCPU, containerIO = c.computeCgroupDeltas(cgroupPaths, now)
	} else {
		// No containers — reset previous samples.
		c.prevCgroupCPU = make(map[string]uint64)
		c.prevCgroupIO = make(map[string]uint64)
	}

	m.ContainerCount = int32(containerCount)
	m.ContainerCpuPercent = containerCPU
	m.ContainerIoBytesPerSec = containerIO

	return m, nil
}

// computeCgroupDeltas reads current CPU/IO from each container cgroup,
// computes rates against previous samples, and stores current values.
// Must be called with c.mu held.
func (c *Collector) computeCgroupDeltas(paths []string, now time.Time) (cpuPercent, ioBytesPerSec float64) {
	dt := now.Sub(c.prevCgroupTime).Seconds()
	hasPrev := len(c.prevCgroupCPU) > 0 && dt > 0

	newCPU := make(map[string]uint64, len(paths))
	newIO := make(map[string]uint64, len(paths))

	for _, p := range paths {
		cpu, err := cgroupCPUUsage(p)
		if err != nil {
			continue
		}
		io := cgroupIOBytes(p)
		newCPU[p] = cpu
		newIO[p] = io

		if hasPrev {
			if prevCPU, ok := c.prevCgroupCPU[p]; ok {
				// usage_usec delta → percentage of wall time.
				cpuDelta := safeDelta(cpu, prevCPU)
				// Convert microseconds to seconds, then to percent of wall time.
				cpuPercent += float64(cpuDelta) / (dt * 1e6) * 100.0
			}
			if prevIO, ok := c.prevCgroupIO[p]; ok {
				ioDelta := safeDelta(io, prevIO)
				ioBytesPerSec += float64(ioDelta) / dt
			}
		}
	}

	c.prevCgroupCPU = newCPU
	c.prevCgroupIO = newIO
	c.prevCgroupTime = now
	return cpuPercent, ioBytesPerSec
}
