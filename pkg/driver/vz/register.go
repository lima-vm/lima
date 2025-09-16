//go:build darwin && !external_vz

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package vz

import (
	"context"

	"github.com/lima-vm/lima/v2/pkg/registry"
)

func init() {
	registry.Register(context.Background(), New())
}
