//go:build darwin && !external_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"github.com/lima-vm/lima/pkg/registry"
)

func init() {
	registry.Register(New())
}
