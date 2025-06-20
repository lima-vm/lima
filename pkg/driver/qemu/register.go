// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package qemu

import (
	"github.com/lima-vm/lima/pkg/registry"
)

func init() {
	registry.Register(New())
}
