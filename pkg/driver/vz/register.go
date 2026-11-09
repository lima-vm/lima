//go:build darwin && !no_vz && !external_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"

	"github.com/lima-vm/lima/v2/pkg/registry"
)

// lima-driver-vz serves New() without this wrapper, so external_vz builds have no USB passthrough.
func init() {
	registry.Register(context.Background(), &usbDriver{LimaVzDriver: New()})
}
