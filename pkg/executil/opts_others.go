//go:build !windows

// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package executil

import (
	"syscall"
)

var (
	ForegroundSysProcAttr = &syscall.SysProcAttr{}
	BackgroundSysProcAttr = &syscall.SysProcAttr{Setpgid: true}
)
