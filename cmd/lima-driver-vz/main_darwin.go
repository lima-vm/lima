// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/lima-vm/lima/v2/pkg/driver/external/server"
	"github.com/lima-vm/lima/v2/pkg/driver/vz"
)

func init() {
	// Lock the main goroutine to the initial thread, which AppKit requires.
	runtime.LockOSThread()
}

// To be used as an external driver for Lima.
func main() {
	execAppBundle()
	vz.RunMainThreadLoop(func() {
		server.Serve(context.Background(), vz.New())
	})
}

// execAppBundle re-executes through a symlink into an app bundle, because
// macOS applies the bundle's Info.plist only to a process run from inside it.
func execAppBundle() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil || resolved == exe || !strings.Contains(resolved, ".app/Contents/MacOS/") {
		return
	}
	// Keep os.Args[0], from which the driver name is derived.
	_ = syscall.Exec(resolved, os.Args, os.Environ())
}
