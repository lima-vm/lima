// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestParseProcMeminfo(t *testing.T) {
	data := `MemTotal:       12288000 kB
MemFree:         1024000 kB
MemAvailable:    6144000 kB
Buffers:          512000 kB
Cached:          3072000 kB
SwapCached:       256000 kB
Active:          4096000 kB
Inactive:        2048000 kB
Active(anon):    3000000 kB
Inactive(anon):  1000000 kB
Active(file):    1096000 kB
Inactive(file):  1048000 kB
SwapTotal:       8192000 kB
SwapFree:        7168000 kB
AnonPages:       3500000 kB
`
	m, err := parseProcMeminfo([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, m.MemTotalBytes, uint64(12288000*1024))
	assert.Equal(t, m.MemAvailableBytes, uint64(6144000*1024))
	assert.Equal(t, m.MemCachedBytes, uint64(3072000*1024))
	assert.Equal(t, m.SwapTotalBytes, uint64(8192000*1024))
	assert.Equal(t, m.SwapFreeBytes, uint64(7168000*1024))
	assert.Equal(t, m.AnonRssBytes, uint64(3500000*1024))
}

func TestParseProcPressureMemory(t *testing.T) {
	data := `some avg10=5.50 avg60=3.20 avg300=1.10 total=123456
full avg10=1.25 avg60=0.80 avg300=0.30 total=789012
`
	some10, full10, err := parseProcPressureMemory([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, some10, 5.50)
	assert.Equal(t, full10, 1.25)
}

func TestParseProcPressureMemory_NoPSI(t *testing.T) {
	// When PSI is not available, return zeros.
	_, _, err := parseProcPressureMemory(nil)
	assert.NilError(t, err)
}

// --- Edge case tests ---

func TestParseProcMeminfo_Empty(t *testing.T) {
	m, err := parseProcMeminfo([]byte(""))
	assert.NilError(t, err)
	assert.Equal(t, m.MemTotalBytes, uint64(0))
	assert.Equal(t, m.AnonRssBytes, uint64(0))
}

func TestParseProcMeminfo_MalformedLines(t *testing.T) {
	// Lines with no value, non-numeric value, single field.
	data := `MemTotal:
MemFree:   notanumber kB
Cached: 1024 kB
justoneword
:
`
	m, err := parseProcMeminfo([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, m.MemCachedBytes, uint64(1024*1024))
	assert.Equal(t, m.MemTotalBytes, uint64(0)) // Could not parse.
}

func TestParseProcMeminfo_MissingFields(t *testing.T) {
	// Only MemTotal present; all other fields stay zero.
	data := `MemTotal:       8000000 kB
`
	m, err := parseProcMeminfo([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, m.MemTotalBytes, uint64(8000000*1024))
	assert.Equal(t, m.MemAvailableBytes, uint64(0))
	assert.Equal(t, m.SwapTotalBytes, uint64(0))
	assert.Equal(t, m.AnonRssBytes, uint64(0))
}

func TestParseProcMeminfo_ExtraWhitespace(t *testing.T) {
	data := `MemTotal:          16384000    kB
AnonPages:         2000000 kB
`
	m, err := parseProcMeminfo([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, m.MemTotalBytes, uint64(16384000*1024))
	assert.Equal(t, m.AnonRssBytes, uint64(2000000*1024))
}

func TestParseProcPressureMemory_Empty(t *testing.T) {
	some10, full10, err := parseProcPressureMemory([]byte(""))
	assert.NilError(t, err)
	assert.Equal(t, some10, 0.0)
	assert.Equal(t, full10, 0.0)
}

func TestParseProcPressureMemory_PartialData(t *testing.T) {
	// Only "some" line, no "full" line.
	data := `some avg10=3.14 avg60=2.00 avg300=1.00 total=99999
`
	some10, full10, err := parseProcPressureMemory([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, some10, 3.14)
	assert.Equal(t, full10, 0.0) // Not present.
}

func TestParseProcPressureMemory_MalformedAvg(t *testing.T) {
	// avg10= has non-numeric value — should be silently skipped.
	data := `some avg10=notfloat avg60=2.00 avg300=1.00 total=99999
full avg10=1.50 avg60=0.80 avg300=0.30 total=789012
`
	some10, full10, err := parseProcPressureMemory([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, some10, 0.0) // Could not parse.
	assert.Equal(t, full10, 1.50)
}

func TestParseProcPressureMemory_NoAvg10Field(t *testing.T) {
	// Lines without avg10= field.
	data := `some avg60=2.00 avg300=1.00 total=99999
full avg60=0.80 avg300=0.30 total=789012
`
	some10, full10, err := parseProcPressureMemory([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, some10, 0.0)
	assert.Equal(t, full10, 0.0)
}

func TestParseProcPressureMemory_ShortLine(t *testing.T) {
	// Line with only one field.
	data := `some
`
	some10, full10, err := parseProcPressureMemory([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, some10, 0.0)
	assert.Equal(t, full10, 0.0)
}

func TestParseProcMeminfo_AllFieldsPopulated(t *testing.T) {
	data := []byte(`MemTotal:       16384000 kB
MemFree:         1024000 kB
MemAvailable:    8192000 kB
Buffers:          512000 kB
Cached:          4096000 kB
SwapTotal:       8192000 kB
SwapFree:        4096000 kB
AnonPages:       2048000 kB
`)
	m, err := parseProcMeminfo(data)
	assert.NilError(t, err)
	assert.Equal(t, m.MemTotalBytes, uint64(16384000*1024))
	assert.Equal(t, m.MemAvailableBytes, uint64(8192000*1024))
	assert.Equal(t, m.MemCachedBytes, uint64(4096000*1024))
	assert.Equal(t, m.SwapTotalBytes, uint64(8192000*1024))
	assert.Equal(t, m.SwapFreeBytes, uint64(4096000*1024))
	assert.Equal(t, m.AnonRssBytes, uint64(2048000*1024))
}

func TestParseProcPressureMemory_BothMissing(t *testing.T) {
	// Data with neither "some" nor "full" lines.
	data := []byte("random_type avg10=1.23 avg60=0.50 avg300=0.25 total=12345\n")
	some10, full10, err := parseProcPressureMemory(data)
	assert.NilError(t, err)
	assert.Equal(t, some10, 0.0)
	assert.Equal(t, full10, 0.0)
}

func TestParseProcPressureMemory_OnlySome(t *testing.T) {
	data := []byte("some avg10=5.50 avg60=3.20 avg300=1.10 total=999\n")
	some10, full10, err := parseProcPressureMemory(data)
	assert.NilError(t, err)
	assert.Equal(t, some10, 5.50)
	assert.Equal(t, full10, 0.0)
}

func TestParseProcPressureMemory_OnlyFull(t *testing.T) {
	data := []byte("full avg10=2.30 avg60=1.50 avg300=0.80 total=555\n")
	some10, full10, err := parseProcPressureMemory(data)
	assert.NilError(t, err)
	assert.Equal(t, some10, 0.0)
	assert.Equal(t, full10, 2.30)
}

// --- /proc/vmstat parsing tests ---

func TestParseProcVmstat(t *testing.T) {
	data := `nr_free_pages 262144
pswpin 1000
pswpout 2500
pgfault 500000
pgmajfault 200
oom_kill 3
nr_dirty 50
`
	vs, err := parseProcVmstat([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, vs.pswpin, uint64(1000))
	assert.Equal(t, vs.pswpout, uint64(2500))
	assert.Equal(t, vs.pgfault, uint64(500000))
	assert.Equal(t, vs.oomKill, uint64(3))
}

func TestParseProcVmstat_Empty(t *testing.T) {
	vs, err := parseProcVmstat([]byte(""))
	assert.NilError(t, err)
	assert.Equal(t, vs.pswpin, uint64(0))
	assert.Equal(t, vs.pswpout, uint64(0))
	assert.Equal(t, vs.pgfault, uint64(0))
	assert.Equal(t, vs.oomKill, uint64(0))
}

func TestParseProcVmstat_MalformedLines(t *testing.T) {
	data := `pswpin notanumber
pswpout 100
singleword
`
	vs, err := parseProcVmstat([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, vs.pswpin, uint64(0))    // Malformed, skipped.
	assert.Equal(t, vs.pswpout, uint64(100)) // Valid.
}

func TestParseProcVmstat_MissingFields(t *testing.T) {
	// Only oom_kill present; swap counters stay zero.
	data := `oom_kill 5
`
	vs, err := parseProcVmstat([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, vs.pswpin, uint64(0))
	assert.Equal(t, vs.oomKill, uint64(5))
}

// --- Collector tests ---

func TestCollector_SwapRates(t *testing.T) {
	c := NewCollector(nil)
	// First sample: sets baseline, rates are zero.
	vs1 := vmstatCounters{pswpin: 100, pswpout: 200, pgfault: 1000}
	now := time.Now()
	c.computeVmstatDeltas(vs1, now)
	assert.Equal(t, c.swapInBytesPerSec, 0.0)
	assert.Equal(t, c.swapOutBytesPerSec, 0.0)

	// Second sample: 10 seconds later, pswpin increased by 50 pages.
	vs2 := vmstatCounters{pswpin: 150, pswpout: 300, pgfault: 2000}
	later := now.Add(10 * time.Second)
	c.computeVmstatDeltas(vs2, later)
	// 50 pages / 10 seconds * 4096 bytes/page = 20480 bytes/sec.
	assert.Equal(t, c.swapInBytesPerSec, 50.0*4096.0/10.0)
	// 100 pages / 10 seconds * 4096 bytes/page = 40960 bytes/sec.
	assert.Equal(t, c.swapOutBytesPerSec, 100.0*4096.0/10.0)
	// 1000 faults / 10 seconds = 100 faults/sec.
	assert.Equal(t, c.pageFaultRate, 100.0)
}

func TestCollector_SwapRates_ZeroDuration(t *testing.T) {
	c := NewCollector(nil)
	vs := vmstatCounters{pswpin: 100, pswpout: 200}
	now := time.Now()
	c.computeVmstatDeltas(vs, now)
	// Same timestamp — should not divide by zero, rates stay at zero.
	c.computeVmstatDeltas(vs, now)
	assert.Equal(t, c.swapInBytesPerSec, 0.0)
}

func TestCollector_OomDetected(t *testing.T) {
	c := NewCollector(nil)
	now := time.Now()
	// First sample: baseline oom_kill=0.
	vs1 := vmstatCounters{oomKill: 0}
	c.computeVmstatDeltas(vs1, now)
	assert.Equal(t, c.oomDetected, false)

	// Second sample: oom_kill increased to 1.
	vs2 := vmstatCounters{oomKill: 1}
	c.computeVmstatDeltas(vs2, now.Add(10*time.Second))
	assert.Equal(t, c.oomDetected, true)

	// Read clears the flag (edge-triggered).
	assert.Equal(t, c.consumeOomDetected(), true)
	assert.Equal(t, c.oomDetected, false)

	// Third sample: oom_kill unchanged — no new OOM.
	vs3 := vmstatCounters{oomKill: 1}
	c.computeVmstatDeltas(vs3, now.Add(20*time.Second))
	assert.Equal(t, c.oomDetected, false)
}

func TestCollector_CounterWrap(t *testing.T) {
	// If counters decrease (e.g., system reboot), treat as reset.
	c := NewCollector(nil)
	now := time.Now()
	vs1 := vmstatCounters{pswpin: 1000, pswpout: 2000, pgfault: 5000, oomKill: 2}
	c.computeVmstatDeltas(vs1, now)

	// Counter decreased — treat delta as zero, not negative.
	vs2 := vmstatCounters{pswpin: 500, pswpout: 800, pgfault: 100, oomKill: 0}
	c.computeVmstatDeltas(vs2, now.Add(10*time.Second))
	assert.Equal(t, c.swapInBytesPerSec, 0.0)
	assert.Equal(t, c.swapOutBytesPerSec, 0.0)
	assert.Equal(t, c.pageFaultRate, 0.0)
	assert.Equal(t, c.oomDetected, false)
}

// --- Docker container stats parsing tests ---

func TestParseDockerContainerList(t *testing.T) {
	data := `[
  {"Id": "abc123", "State": "running"},
  {"Id": "def456", "State": "running"},
  {"Id": "ghi789", "State": "exited"}
]`
	ids, err := parseDockerContainerList([]byte(data))
	assert.NilError(t, err)
	// Only running containers.
	assert.Equal(t, len(ids), 2)
}

func TestParseDockerContainerList_Empty(t *testing.T) {
	ids, err := parseDockerContainerList([]byte("[]"))
	assert.NilError(t, err)
	assert.Equal(t, len(ids), 0)
}

func TestParseDockerStats(t *testing.T) {
	// Simplified Docker stats API response (single-shot, stream=false).
	data := `{
  "cpu_stats": {
    "cpu_usage": {"total_usage": 200000000},
    "system_cpu_usage": 1000000000,
    "online_cpus": 4
  },
  "precpu_stats": {
    "cpu_usage": {"total_usage": 100000000},
    "system_cpu_usage": 900000000,
    "online_cpus": 4
  },
  "blkio_stats": {
    "io_service_bytes_recursive": [
      {"op": "read", "value": 5000},
      {"op": "write", "value": 3000}
    ]
  }
}`
	cpuPct, ioBytes, err := parseDockerStats([]byte(data))
	assert.NilError(t, err)
	// CPU delta = 200M - 100M = 100M; system delta = 1000M - 900M = 100M.
	// CPU% = (100M / 100M) * 4 cpus * 100 = 400%.
	assert.Equal(t, cpuPct, 400.0)
	// IO bytes = 5000 + 3000 = 8000.
	assert.Equal(t, ioBytes, uint64(8000))
}

func TestParseDockerStats_NoPrecpu(t *testing.T) {
	// When precpu_stats is empty, CPU% should be 0.
	data := `{
  "cpu_stats": {
    "cpu_usage": {"total_usage": 200000000},
    "system_cpu_usage": 1000000000,
    "online_cpus": 4
  },
  "precpu_stats": {
    "cpu_usage": {"total_usage": 0},
    "system_cpu_usage": 0,
    "online_cpus": 0
  }
}`
	cpuPct, _, err := parseDockerStats([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, cpuPct, 0.0)
}

func TestParseDockerStats_NoBlkio(t *testing.T) {
	data := `{
  "cpu_stats": {
    "cpu_usage": {"total_usage": 200000000},
    "system_cpu_usage": 1000000000,
    "online_cpus": 4
  },
  "precpu_stats": {
    "cpu_usage": {"total_usage": 100000000},
    "system_cpu_usage": 900000000,
    "online_cpus": 4
  }
}`
	_, ioBytes, err := parseDockerStats([]byte(data))
	assert.NilError(t, err)
	assert.Equal(t, ioBytes, uint64(0))
}

func TestParseDockerStats_CPUCounterWrap(t *testing.T) {
	// When a container restarts, current TotalUsage can be less than PreCPU.
	// The uint64 subtraction must not wrap around to a huge value.
	data := []byte(`{
		"cpu_stats": {"cpu_usage": {"total_usage": 50000}, "system_cpu_usage": 2000000000, "online_cpus": 4},
		"precpu_stats": {"cpu_usage": {"total_usage": 100000000}, "system_cpu_usage": 1000000000, "online_cpus": 4}
	}`)
	cpuPct, _, err := parseDockerStats(data)
	assert.NilError(t, err)
	// CPU should be 0 (not astronomical) when counters wrap.
	assert.Equal(t, cpuPct, float64(0))
}

func TestParseDockerStats_SystemCounterWrap(t *testing.T) {
	// When system counters wrap (e.g., after host reboot), CPU should be 0.
	data := []byte(`{
		"cpu_stats": {"cpu_usage": {"total_usage": 200000000}, "system_cpu_usage": 500000000, "online_cpus": 4},
		"precpu_stats": {"cpu_usage": {"total_usage": 100000000}, "system_cpu_usage": 1000000000, "online_cpus": 4}
	}`)
	cpuPct, _, err := parseDockerStats(data)
	assert.NilError(t, err)
	assert.Equal(t, cpuPct, float64(0))
}
