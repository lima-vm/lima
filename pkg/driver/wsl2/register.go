//go:build windows && !no_wsl

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package wsl2

import (
	"github.com/lima-vm/lima/pkg/registry"
)

func init() {
	registry.Register(New())
}
