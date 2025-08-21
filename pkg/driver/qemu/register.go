//go:build !external_qemu

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"context"

	"github.com/lima-vm/lima/v2/pkg/registry"
)

func init() {
	registry.Register(context.Background(), New())
}
