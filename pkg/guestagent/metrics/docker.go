// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"encoding/json"
)

// dockerContainer is a minimal representation of the Docker API
// /containers/json response.
type dockerContainer struct {
	ID    string `json:"Id"`
	State string `json:"State"`
}

// parseDockerContainerList parses the JSON response from Docker's
// /containers/json endpoint and returns IDs of running containers.
func parseDockerContainerList(data []byte) ([]string, error) {
	var containers []dockerContainer
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, err
	}
	var ids []string
	for _, c := range containers {
		// Defensive: re-check state in case the API contract changes.
		// The request already filters by status=["running"].
		if c.State == "running" {
			ids = append(ids, c.ID)
		}
	}
	return ids, nil
}

// dockerStatsResponse is a minimal representation of the Docker API
// /containers/{id}/stats?stream=false response.
type dockerStatsResponse struct {
	CPUStats    dockerCPUStats   `json:"cpu_stats"`
	PreCPUStats dockerCPUStats   `json:"precpu_stats"`
	BlkioStats  dockerBlkioStats `json:"blkio_stats"`
}

type dockerCPUStats struct {
	CPUUsage    dockerCPUUsage `json:"cpu_usage"`
	SystemUsage uint64         `json:"system_cpu_usage"`
	OnlineCPUs  int            `json:"online_cpus"`
}

type dockerCPUUsage struct {
	TotalUsage uint64 `json:"total_usage"`
}

type dockerBlkioStats struct {
	IOServiceBytesRecursive []dockerBlkioEntry `json:"io_service_bytes_recursive"`
}

type dockerBlkioEntry struct {
	Op    string `json:"op"`
	Value uint64 `json:"value"`
}

// parseDockerStats parses a single Docker stats JSON response and returns
// CPU percentage and total IO bytes.
//
// CPU% is calculated the same way as `docker stats`:
//
//	cpuDelta / systemDelta * onlineCPUs * 100.
func parseDockerStats(data []byte) (cpuPercent float64, ioBytes uint64, err error) {
	var stats dockerStatsResponse
	if err := json.Unmarshal(data, &stats); err != nil {
		return 0, 0, err
	}

	// CPU percentage calculation (matches docker/cli formula).
	// Guard against uint64 underflow: when a container restarts, current counters
	// can be smaller than previous ones, causing wrap-around to huge values.
	if stats.PreCPUStats.SystemUsage > 0 &&
		stats.CPUStats.CPUUsage.TotalUsage >= stats.PreCPUStats.CPUUsage.TotalUsage &&
		stats.CPUStats.SystemUsage >= stats.PreCPUStats.SystemUsage {
		cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
		if systemDelta > 0 && cpuDelta > 0 {
			cpuPercent = (cpuDelta / systemDelta) * float64(stats.CPUStats.OnlineCPUs) * 100.0
		}
	}

	// Total IO bytes (read + write).
	for _, entry := range stats.BlkioStats.IOServiceBytesRecursive {
		ioBytes += entry.Value
	}

	return cpuPercent, ioBytes, nil
}
