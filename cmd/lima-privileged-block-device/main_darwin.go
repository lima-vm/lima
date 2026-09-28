//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"github.com/lima-vm/lima/v2/pkg/blockdevice"
	"github.com/lima-vm/lima/v2/pkg/osutil"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s DEVICE\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}
	err := blockdevice.ServeSudoOpenBlockDevice(os.Args[1], os.Stdin)
	osutil.HandleExitError(err)
	if err != nil {
		logrus.Fatal(err)
	}
}
