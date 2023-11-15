//go:build !external_ext

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package ext

import "github.com/lima-vm/lima/v2/pkg/registry"

func init() {
	registry.Register(New())
}
