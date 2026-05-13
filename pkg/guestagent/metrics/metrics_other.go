// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

// Package metrics reads guest memory statistics from /proc for the balloon controller.
// On non-Linux platforms, all functions return errors since /proc is not available.
package metrics

import (
	"context"
	"errors"

	"github.com/lima-vm/lima/v2/pkg/guestagent/api"
)

// Collector is a stateful memory metrics collector. On non-Linux platforms
// it is a no-op stub.
type Collector struct{}

// NewCollector creates a no-op collector on non-Linux platforms.
func NewCollector() *Collector {
	return &Collector{}
}

// Collect returns an error on non-Linux platforms.
func (c *Collector) Collect(_ context.Context) (*api.MemoryMetrics, error) {
	return nil, errors.New("memory metrics collection requires Linux")
}

// CollectMemoryMetrics returns an error on non-Linux platforms.
func CollectMemoryMetrics() (*api.MemoryMetrics, error) {
	return nil, errors.New("memory metrics collection requires Linux")
}
