// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

const pageSize = 4096 // Linux page size in bytes.

// Collector is a stateful memory metrics collector that tracks deltas
// between successive /proc/vmstat samples and queries Docker for
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

	// Docker socket path (nil disables container stats).
	dockerSocket string
	httpClient   *http.Client
}

// NewCollector creates a new stateful metrics collector.
// If dockerSocket is nil or empty, container stats collection is disabled.
func NewCollector(dockerSocket *string) *Collector {
	c := &Collector{}
	if dockerSocket != nil && *dockerSocket != "" {
		c.dockerSocket = *dockerSocket
		c.httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", c.dockerSocket)
				},
			},
			Timeout: 5 * time.Second,
		}
	}
	return c
}

// Close releases resources held by the collector (e.g., HTTP transport).
func (c *Collector) Close() {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
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
func (c *Collector) Collect() (*api.MemoryMetrics, error) {
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
	some10, full10, parseErr := parseProcPressureMemory(pressure)
	if parseErr != nil {
		return nil, parseErr
	}
	m.PsiMemorySome_10 = some10
	m.PsiMemoryFull_10 = full10

	// 2. /proc/vmstat for swap rates, page faults, OOM.
	vmstatData, vmstatErr := os.ReadFile("/proc/vmstat")

	// 3. Docker container stats (best-effort, outside lock — httpClient is immutable).
	var dockerCount int
	var dockerCPU, dockerIO float64
	if c.httpClient != nil {
		dockerCount, dockerCPU, dockerIO = c.collectDockerStats()
	}

	// Hold lock only for internal state updates and reads.
	c.mu.Lock()
	defer c.mu.Unlock()

	if vmstatErr == nil {
		vs, parseErr := parseProcVmstat(vmstatData)
		if parseErr == nil {
			c.computeVmstatDeltas(vs, time.Now())
		}
	}
	m.SwapInBytesPerSec = c.swapInBytesPerSec
	m.SwapOutBytesPerSec = c.swapOutBytesPerSec
	m.PageFaultRate = c.pageFaultRate
	m.OomDetected = c.consumeOomDetected()

	m.ContainerCount = int32(dockerCount)
	m.ContainerCpuPercent = dockerCPU
	m.ContainerIoBytesPerSec = dockerIO

	return m, nil
}

// collectDockerStats queries the Docker socket for container count,
// aggregate CPU%, and aggregate IO bytes/sec. Returns zeros on error.
func (c *Collector) collectDockerStats() (count int, cpuPercent, ioBytesPerSec float64) {
	// List running containers.
	listReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		"http://localhost/containers/json?filters=%7B%22status%22%3A%5B%22running%22%5D%7D", http.NoBody)
	if err != nil {
		logrus.Debugf("Docker stats: failed to create list request: %v", err)
		return 0, 0, 0
	}
	resp, err := c.httpClient.Do(listReq)
	if err != nil {
		logrus.Debugf("Docker stats: failed to list containers: %v", err)
		return 0, 0, 0
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, 0
	}

	ids, err := parseDockerContainerList(body)
	if err != nil {
		logrus.Debugf("Docker stats: failed to parse container list: %v", err)
		return 0, 0, 0
	}
	count = len(ids)
	if count == 0 {
		return 0, 0, 0
	}

	// Aggregate stats from each container (best-effort, skip failures).
	var totalCPU float64
	var totalIO uint64
	for _, id := range ids {
		statsReq, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet,
			"http://localhost/containers/"+id+"/stats?stream=false&one-shot=true", http.NoBody)
		if reqErr != nil {
			continue
		}
		statsResp, doErr := c.httpClient.Do(statsReq)
		if doErr != nil {
			continue
		}
		statsBody, readErr := io.ReadAll(statsResp.Body)
		statsResp.Body.Close()
		if readErr != nil {
			continue
		}
		cpuPct, ioBytes, parseErr := parseDockerStats(statsBody)
		if parseErr != nil {
			continue
		}
		totalCPU += cpuPct
		totalIO += ioBytes
	}

	return count, totalCPU, float64(totalIO)
}
