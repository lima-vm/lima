// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package builtins

import (
	// Import all built-in drivers to register them in the registry.
	_ "github.com/lima-vm/lima/pkg/qemu"
	_ "github.com/lima-vm/lima/pkg/vz"
	_ "github.com/lima-vm/lima/pkg/wsl2"
)
