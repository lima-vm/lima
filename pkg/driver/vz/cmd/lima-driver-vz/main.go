//go:build (darwin && amd64) || (darwin && arm64)

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/lima-vm/lima/pkg/driver/external/server"
	"github.com/lima-vm/lima/pkg/driver/vz"
)

// To be used as an external driver for Lima.
func main() {
	driver := vz.New()
	server.Serve(driver)
}
