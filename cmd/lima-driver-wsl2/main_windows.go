// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/lima-vm/lima/v2/pkg/driver/external/server"
	"github.com/lima-vm/lima/v2/pkg/driver/wsl2"
)

// To be used as an external driver for Lima.
func main() {
	server.Serve(wsl2.New())
}
