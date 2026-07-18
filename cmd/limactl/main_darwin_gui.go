//go:build darwin

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package main

import "runtime"

func init() {
	// Lock the main goroutine to the initial process thread before main() runs.
	// VZVirtualMachineView requires GUI operations on the initial thread; without
	// this, Go's scheduler may migrate the goroutine before hostagent starts.
	// Assumes no other package init() triggers a scheduling point before this runs.
	runtime.LockOSThread()
}
