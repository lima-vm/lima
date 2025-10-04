// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package krun

import "github.com/lima-vm/lima/v2/pkg/registry"

func init() {
	registry.Register(New())
}
