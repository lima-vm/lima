// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/lima-vm/lima/pkg/driver/external/server"
	"github.com/lima-vm/lima/pkg/driver/qemu"
)

// To be used as an external driver for Lima.
func main() {
	driver := qemu.New()
	server.Serve(driver)
}
